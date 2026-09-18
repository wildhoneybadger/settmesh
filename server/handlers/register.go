package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

type registerRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

const (
	minUsernameLen = 3
	maxUsernameLen = 32
	minPasswordLen = 8
)

// Register crée un nouveau compte (identifiant + mot de passe, sans email ni
// téléphone) à partir d'un code d'invitation valide et non utilisé.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	username := strings.TrimSpace(req.Username)
	code := strings.TrimSpace(req.InviteCode)

	if !isValidUsername(username) {
		writeError(w, http.StatusBadRequest, "invalid username")
		return
	}
	if len(req.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "password too short")
		return
	}
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code required")
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	id, err := auth.GenerateID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	err = s.DB.CreateUserWithInvite(r.Context(), id, username, passwordHash, code, time.Now().UTC())
	switch {
	case errors.Is(err, storage.ErrInviteInvalid):
		writeError(w, http.StatusForbidden, "invalid or expired invite code")
		return
	case errors.Is(err, storage.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username already taken")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":       id,
		"username": username,
	})
}

func isValidUsername(u string) bool {
	if len(u) < minUsernameLen || len(u) > maxUsernameLen {
		return false
	}
	for _, r := range u {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isUpper && !isDigit && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
