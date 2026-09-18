package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

const (
	// maxFileSizeBytes est la limite par fichier imposée par CLAUDE.md (20 Mo).
	maxFileSizeBytes = 20 * 1024 * 1024
	// maxPendingFileBytesPerUser plafonne le volume total de photos en attente par
	// compte, pour gérer les destinataires restant hors ligne longtemps.
	maxPendingFileBytesPerUser = 200 * 1024 * 1024
)

// UploadFile stocke le contenu chiffré d'une photo à destination d'un autre
// utilisateur (corps de requête = octets chiffrés bruts). Le serveur ne voit qu'un
// blob opaque : la clé de déchiffrement symétrique est transmise séparément,
// chiffrée de bout en bout, via un message classique (POST /messages).
func (s *Server) UploadFile(w http.ResponseWriter, r *http.Request) {
	recipientUsername := r.URL.Query().Get("recipient")
	if recipientUsername == "" {
		writeError(w, http.StatusBadRequest, "recipient required")
		return
	}

	recipient, err := s.DB.GetUserByUsername(r.Context(), recipientUsername)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "recipient not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	pendingBytes, err := s.DB.SumPendingFileBytes(r.Context(), recipient.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if pendingBytes >= maxPendingFileBytesPerUser {
		writeError(w, http.StatusInsufficientStorage, "recipient storage quota exceeded")
		return
	}

	id, err := auth.GenerateID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxFileSizeBytes)
	size, err := s.Files.Write(id, body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if size == 0 {
		_ = s.Files.Delete(id)
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}

	senderID := userIDFromContext(r)
	now := time.Now().UTC()
	expiresAt := now.Add(s.FileTTL)
	if err := s.DB.CreateFile(r.Context(), id, senderID, recipient.ID, size, now, expiresAt); err != nil {
		_ = s.Files.Delete(id)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         id,
		"size_bytes": size,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// DownloadFile retourne le contenu chiffré d'un fichier à son destinataire
// authentifié. Ne le supprime pas : le client doit en accuser réception
// (DELETE /files/{id}) une fois qu'il l'a déchiffré et persisté localement.
func (s *Server) DownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := userIDFromContext(r)

	meta, err := s.DB.GetFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if meta.RecipientID != userID {
		// Ne pas distinguer "existe mais n'est pas à toi" de "n'existe pas".
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	f, err := s.Files.Open(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(meta.SizeBytes, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// AckFile accuse réception d'un fichier et le supprime définitivement du serveur,
// métadonnées et octets chiffrés sur disque.
func (s *Server) AckFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := userIDFromContext(r)

	if err := s.DB.DeleteFile(r.Context(), id, userID); err != nil {
		if errors.Is(err, storage.ErrFileNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	_ = s.Files.Delete(id)
	writeNoContent(w)
}
