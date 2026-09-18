package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/wildhoneybadger/settmesh/server/storage"
)

type identityKeyRequest struct {
	PublicKey string `json:"public_key"`
}

// UploadIdentityKey enregistre (ou remplace) la clé d'identité publique à long terme
// de l'utilisateur authentifié. La clé privée correspondante ne quitte jamais l'appareil.
func (s *Server) UploadIdentityKey(w http.ResponseWriter, r *http.Request) {
	var req identityKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !decodeFixedLenKey(req.PublicKey, curve25519KeyLen) {
		writeError(w, http.StatusBadRequest, "invalid public key")
		return
	}

	userID := userIDFromContext(r)
	if err := s.DB.UpsertIdentityKey(r.Context(), userID, req.PublicKey, time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeNoContent(w)
}

type signedPreKeyRequest struct {
	KeyID     int    `json:"key_id"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

// UploadSignedPreKey enregistre (ou remplace) la prekey signée courante de
// l'utilisateur authentifié, à rotater périodiquement côté client.
func (s *Server) UploadSignedPreKey(w http.ResponseWriter, r *http.Request) {
	var req signedPreKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.KeyID < 0 {
		writeError(w, http.StatusBadRequest, "invalid key id")
		return
	}
	if !decodeFixedLenKey(req.PublicKey, curve25519KeyLen) {
		writeError(w, http.StatusBadRequest, "invalid public key")
		return
	}
	if !decodeFixedLenKey(req.Signature, signatureLen) {
		writeError(w, http.StatusBadRequest, "invalid signature")
		return
	}

	userID := userIDFromContext(r)
	err := s.DB.UpsertSignedPreKey(r.Context(), userID, req.KeyID, req.PublicKey, req.Signature, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeNoContent(w)
}

type oneTimePreKeysRequest struct {
	Keys []struct {
		KeyID     int    `json:"key_id"`
		PublicKey string `json:"public_key"`
	} `json:"keys"`
}

// maxOneTimePreKeysPerUpload borne la taille d'un lot pour éviter un abus de stockage.
const maxOneTimePreKeysPerUpload = 200

// UploadOneTimePreKeys ajoute un lot de prekeys à usage unique, pour reconstituer le
// stock disponible côté serveur quand le client en a consommé une partie.
func (s *Server) UploadOneTimePreKeys(w http.ResponseWriter, r *http.Request) {
	var req oneTimePreKeysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Keys) == 0 || len(req.Keys) > maxOneTimePreKeysPerUpload {
		writeError(w, http.StatusBadRequest, "invalid number of keys")
		return
	}

	keys := make([]storage.OneTimePreKeyInput, 0, len(req.Keys))
	for _, k := range req.Keys {
		if k.KeyID < 0 || !decodeFixedLenKey(k.PublicKey, curve25519KeyLen) {
			writeError(w, http.StatusBadRequest, "invalid public key")
			return
		}
		keys = append(keys, storage.OneTimePreKeyInput{KeyID: k.KeyID, PublicKey: k.PublicKey})
	}

	userID := userIDFromContext(r)
	if err := s.DB.InsertOneTimePreKeys(r.Context(), userID, keys, time.Now().UTC()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeNoContent(w)
}

// OneTimePreKeyCount retourne le nombre de prekeys à usage unique encore disponibles
// pour l'utilisateur authentifié, afin qu'il sache quand en reconstituer le stock.
func (s *Server) OneTimePreKeyCount(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r)
	count, err := s.DB.CountUnusedOneTimePreKeys(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

// KeyBundle expose le bundle de clés publiques d'un utilisateur nécessaire pour
// initier une session Signal (X3DH) : clé d'identité, prekey signée, et une prekey à
// usage unique (consommée immédiatement pour ne jamais être distribuée deux fois).
// Si le stock de prekeys à usage unique est épuisé, le bundle est renvoyé sans (X3DH
// reste possible, avec une garantie de forward secrecy légèrement réduite).
func (s *Server) KeyBundle(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	target, err := s.DB.GetUserByUsername(r.Context(), username)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	identity, err := s.DB.GetIdentityKey(r.Context(), target.ID)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			writeError(w, http.StatusNotFound, "no keys published for this user")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	signedPreKey, err := s.DB.GetSignedPreKey(r.Context(), target.ID)
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			writeError(w, http.StatusNotFound, "no keys published for this user")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	oneTime, err := s.DB.ConsumeOneTimePreKey(r.Context(), target.ID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	bundle := map[string]any{
		"username":     target.Username,
		"identity_key": identity.PublicKey,
		"signed_prekey": map[string]any{
			"key_id":     signedPreKey.KeyID,
			"public_key": signedPreKey.PublicKey,
			"signature":  signedPreKey.Signature,
		},
	}
	if oneTime != nil {
		bundle["one_time_prekey"] = map[string]any{
			"key_id":     oneTime.KeyID,
			"public_key": oneTime.PublicKey,
		}
	}

	writeJSON(w, http.StatusOK, bundle)
}
