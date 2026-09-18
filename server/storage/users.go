package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrUsernameTaken est retourné quand l'identifiant demandé est déjà utilisé.
var ErrUsernameTaken = errors.New("identifiant déjà utilisé")

// ErrUserNotFound est retourné quand aucun utilisateur ne correspond à l'identifiant.
var ErrUserNotFound = errors.New("utilisateur introuvable")

// User représente un compte enregistré sur le serveur.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// GetUserByUsername récupère un utilisateur par son identifiant.
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE username = ?`,
		username,
	)

	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}
