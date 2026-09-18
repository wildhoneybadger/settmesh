package storage

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ErrInviteInvalid est retourné quand le code d'invitation est inexistant, expiré ou déjà utilisé.
var ErrInviteInvalid = errors.New("code d'invitation invalide ou déjà utilisé")

// CreateInviteCode enregistre un nouveau code d'invitation valable jusqu'à expiresAt.
func (db *DB) CreateInviteCode(ctx context.Context, code string, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO invite_codes (code, created_at, expires_at) VALUES (?, ?, ?)`,
		code, time.Now().UTC(), expiresAt,
	)
	return err
}

// CreateUserWithInvite crée un nouvel utilisateur et consomme le code d'invitation de
// façon atomique dans une même transaction : soit les deux opérations réussissent
// ensemble, soit aucune n'est appliquée. Un code ne peut donc jamais être consommé sans
// qu'un compte existe réellement, même en cas d'échec partiel (identifiant déjà pris,
// erreur base de données, etc.) pendant le traitement de l'inscription.
func (db *DB) CreateUserWithInvite(ctx context.Context, id, username, passwordHash, code string, now time.Time) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE invite_codes SET used_at = ?, used_by = ?
		 WHERE code = ? AND used_at IS NULL AND expires_at > ?`,
		now, id, code, now,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrInviteInvalid
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		id, username, passwordHash, now,
	); err != nil {
		if isUniqueConstraintErr(err) {
			return ErrUsernameTaken
		}
		return err
	}

	return tx.Commit()
}

func isUniqueConstraintErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
