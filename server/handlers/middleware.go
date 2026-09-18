package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/wildhoneybadger/settmesh/server/storage"
)

type contextKey int

const userIDContextKey contextKey = iota

// RequireAuth protège un handler en exigeant un jeton de session valide
// (en-tête "Authorization: Bearer <token>") et injecte l'identifiant de
// l'utilisateur authentifié dans le contexte de la requête.
func (s *Server) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		token := strings.TrimPrefix(header, prefix)

		session, err := s.DB.GetSession(r.Context(), token)
		if err != nil {
			if !errors.Is(err, storage.ErrSessionNotFound) {
				writeError(w, http.StatusInternalServerError, "internal error")
				return
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if session.ExpiresAt.Before(time.Now()) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, session.UserID)
		next(w, r.WithContext(ctx))
	}
}

func userIDFromContext(r *http.Request) string {
	id, _ := r.Context().Value(userIDContextKey).(string)
	return id
}
