package handlers

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// dummyPasswordHash est comparé aux mots de passe soumis pour un identifiant inconnu,
// afin que la durée de réponse ne révèle pas si le compte existe.
var dummyPasswordHash = mustHash("settmesh-dummy-password-for-timing-mitigation")

func mustHash(password string) string {
	hash, err := auth.HashPassword(password)
	if err != nil {
		panic(err)
	}
	return hash
}

// Login vérifie l'identifiant et le mot de passe, et applique une limitation des
// tentatives par adresse IP avec délai progressif après échecs répétés.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	if allowed, retryAfter := s.LoginLimiter.Allow(ip); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds(retryAfter)))
		writeError(w, http.StatusTooManyRequests, "too many failed attempts, try again later")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.DB.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		if !errors.Is(err, storage.ErrUserNotFound) {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		auth.VerifyPassword(dummyPasswordHash, req.Password)
		s.LoginLimiter.RecordFailure(ip)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		s.LoginLimiter.RecordFailure(ip)
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := auth.GenerateSessionToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	expiresAt := time.Now().UTC().Add(s.SessionTTL)
	if err := s.DB.CreateSession(r.Context(), token, user.ID, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	s.LoginLimiter.RecordSuccess(ip)
	writeJSON(w, http.StatusOK, map[string]string{
		"id":         user.ID,
		"username":   user.Username,
		"token":      token,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

// clientIP retourne l'adresse IP réelle du client. Le serveur n'est joignable que
// depuis le reverse proxy nginx (réseau interne Docker, jamais exposé directement à
// l'extérieur), qui fixe X-Real-IP à l'adresse du client TLS ; cet en-tête peut donc
// être utilisé sans risque de spoofing par un client externe.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func retryAfterSeconds(d time.Duration) int {
	secs := int(d.Round(time.Second).Seconds())
	if secs < 1 {
		secs = 1
	}
	return secs
}
