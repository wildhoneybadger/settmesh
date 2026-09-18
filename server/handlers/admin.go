package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
)

// CreateInviteCode génère un nouveau code d'invitation à usage unique. Réservé à
// l'administrateur du serveur, authentifié par un jeton statique (variable
// d'environnement ADMIN_TOKEN) transmis en en-tête "Authorization: Bearer <token>".
func (s *Server) CreateInviteCode(w http.ResponseWriter, r *http.Request) {
	if s.AdminToken == "" || !isAuthorizedAdmin(r, s.AdminToken) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	code, err := auth.GenerateInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	expiresAt := time.Now().UTC().Add(s.InviteTTL)
	if err := s.DB.CreateInviteCode(r.Context(), code, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"code":       code,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func isAuthorizedAdmin(r *http.Request, token string) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimPrefix(header, prefix)
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}
