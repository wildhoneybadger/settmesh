package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

const (
	// maxMessageSizeBytes borne la taille d'un message texte chiffré. Les fichiers
	// (photos, step 6) transiteront par un flux séparé avec sa propre limite.
	maxMessageSizeBytes = 64 * 1024
	// maxPendingMessagesPerUser est le quota de messages non livrés par compte,
	// pour éviter qu'un destinataire resté hors ligne longtemps n'accumule un stock
	// illimité côté serveur.
	maxPendingMessagesPerUser = 1000
	defaultMessageListLimit   = 100
	maxMessageListLimit       = 500
)

type sendMessageRequest struct {
	Recipient  string `json:"recipient"`
	Ciphertext string `json:"ciphertext"`
}

// SendMessage dépose un message chiffré de bout en bout dans la boîte du
// destinataire. Le serveur ne voit et ne comprend jamais le contenu : il stocke des
// octets chiffrés opaques jusqu'à leur livraison, ou leur expiration automatique.
func (s *Server) SendMessage(w http.ResponseWriter, r *http.Request) {
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ciphertext, err := base64.StdEncoding.DecodeString(req.Ciphertext)
	if err != nil || len(ciphertext) == 0 || len(ciphertext) > maxMessageSizeBytes {
		writeError(w, http.StatusBadRequest, "invalid ciphertext")
		return
	}

	recipient, err := s.DB.GetUserByUsername(r.Context(), req.Recipient)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "recipient not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	pending, err := s.DB.CountPendingMessages(r.Context(), recipient.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if pending >= maxPendingMessagesPerUser {
		writeError(w, http.StatusInsufficientStorage, "recipient mailbox full")
		return
	}

	id, err := auth.GenerateID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	senderID := userIDFromContext(r)
	now := time.Now().UTC()
	expiresAt := now.Add(s.MessageTTL)
	if err := s.DB.CreateMessage(r.Context(), id, senderID, recipient.ID, ciphertext, now, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         id,
		"created_at": now.Format(time.RFC3339),
	})
}

// ListMessages retourne les messages en attente adressés à l'utilisateur
// authentifié, du plus ancien au plus récent. Elle ne les supprime pas : le client
// doit accuser réception de chaque message (DELETE /messages/{id}) une fois qu'il l'a
// déchiffré et persisté localement.
func (s *Server) ListMessages(w http.ResponseWriter, r *http.Request) {
	limit := defaultMessageListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	if limit > maxMessageListLimit {
		limit = maxMessageListLimit
	}

	userID := userIDFromContext(r)
	messages, err := s.DB.ListPendingMessages(r.Context(), userID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]map[string]string, 0, len(messages))
	for _, m := range messages {
		out = append(out, map[string]string{
			"id":         m.ID,
			"sender":     m.SenderUsername,
			"ciphertext": base64.StdEncoding.EncodeToString(m.Ciphertext),
			"sent_at":    m.CreatedAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// AckMessage accuse réception d'un message et le supprime définitivement du serveur.
func (s *Server) AckMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID := userIDFromContext(r)

	if err := s.DB.DeleteMessage(r.Context(), id, userID); err != nil {
		if errors.Is(err, storage.ErrMessageNotFound) {
			writeError(w, http.StatusNotFound, "message not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeNoContent(w)
}
