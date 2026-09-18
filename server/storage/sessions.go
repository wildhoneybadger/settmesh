package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrSessionNotFound est retourné quand le jeton de session est inconnu.
var ErrSessionNotFound = errors.New("session introuvable")

// Session représente une session authentifiée, identifiée par un jeton opaque.
type Session struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// CreateSession enregistre une nouvelle session pour userID, valable jusqu'à expiresAt.
func (db *DB) CreateSession(ctx context.Context, token, userID string, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		token, userID, time.Now().UTC(), expiresAt,
	)
	return err
}

// GetSession récupère une session par son jeton.
func (db *DB) GetSession(ctx context.Context, token string) (*Session, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ?`, token)

	var s Session
	if err := row.Scan(&s.Token, &s.UserID, &s.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

// DeleteExpiredSessions supprime les sessions expirées avant before, pour éviter une
// croissance illimitée de la table au fil du temps.
func (db *DB) DeleteExpiredSessions(ctx context.Context, before time.Time) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, before)
	return err
}
