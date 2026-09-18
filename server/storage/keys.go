package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrKeyNotFound est retourné quand aucune clé n'a encore été publiée par l'utilisateur.
var ErrKeyNotFound = errors.New("clé introuvable")

// IdentityKey est la clé d'identité publique à long terme d'un utilisateur.
type IdentityKey struct {
	UserID    string
	PublicKey string
	UpdatedAt time.Time
}

// UpsertIdentityKey enregistre ou remplace la clé d'identité publique de userID.
func (db *DB) UpsertIdentityKey(ctx context.Context, userID, publicKey string, now time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO identity_keys (user_id, public_key, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET public_key = excluded.public_key, updated_at = excluded.updated_at`,
		userID, publicKey, now,
	)
	return err
}

// GetIdentityKey récupère la clé d'identité publique de userID.
func (db *DB) GetIdentityKey(ctx context.Context, userID string) (*IdentityKey, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT user_id, public_key, updated_at FROM identity_keys WHERE user_id = ?`, userID)

	var k IdentityKey
	if err := row.Scan(&k.UserID, &k.PublicKey, &k.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return &k, nil
}

// SignedPreKey est la prekey signée courante d'un utilisateur, rotatée
// périodiquement côté client.
type SignedPreKey struct {
	UserID    string
	KeyID     int
	PublicKey string
	Signature string
	UpdatedAt time.Time
}

// UpsertSignedPreKey enregistre ou remplace la prekey signée courante de userID.
func (db *DB) UpsertSignedPreKey(ctx context.Context, userID string, keyID int, publicKey, signature string, now time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO signed_prekeys (user_id, key_id, public_key, signature, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id) DO UPDATE SET key_id = excluded.key_id, public_key = excluded.public_key,
		 signature = excluded.signature, updated_at = excluded.updated_at`,
		userID, keyID, publicKey, signature, now,
	)
	return err
}

// GetSignedPreKey récupère la prekey signée courante de userID.
func (db *DB) GetSignedPreKey(ctx context.Context, userID string) (*SignedPreKey, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT user_id, key_id, public_key, signature, updated_at FROM signed_prekeys WHERE user_id = ?`, userID)

	var k SignedPreKey
	if err := row.Scan(&k.UserID, &k.KeyID, &k.PublicKey, &k.Signature, &k.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return &k, nil
}

// OneTimePreKeyInput est une prekey à usage unique fournie par le client ou
// distribuée à un pair.
type OneTimePreKeyInput struct {
	KeyID     int
	PublicKey string
}

// InsertOneTimePreKeys ajoute un lot de prekeys à usage unique pour reconstituer le
// stock côté serveur d'un utilisateur.
func (db *DB) InsertOneTimePreKeys(ctx context.Context, userID string, keys []OneTimePreKeyInput, now time.Time) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO one_time_prekeys (user_id, key_id, public_key, created_at) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, k := range keys {
		if _, err := stmt.ExecContext(ctx, userID, k.KeyID, k.PublicKey, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// CountUnusedOneTimePreKeys retourne le nombre de prekeys à usage unique encore
// disponibles pour userID.
func (db *DB) CountUnusedOneTimePreKeys(ctx context.Context, userID string) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM one_time_prekeys WHERE user_id = ? AND used_at IS NULL`, userID,
	).Scan(&count)
	return count, err
}

// ConsumeOneTimePreKey marque atomiquement la plus ancienne prekey à usage unique
// disponible de userID comme utilisée et la retourne, afin qu'elle ne soit jamais
// distribuée deux fois. Retourne (nil, nil) s'il n'en reste aucune. La connexion à la
// base est limitée à une seule connexion ouverte (voir Open), ce qui garantit que
// cette lecture-puis-écriture ne peut pas être concurrencée par une autre requête.
func (db *DB) ConsumeOneTimePreKey(ctx context.Context, userID string, now time.Time) (*OneTimePreKeyInput, error) {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`SELECT key_id, public_key FROM one_time_prekeys
		 WHERE user_id = ? AND used_at IS NULL ORDER BY key_id LIMIT 1`, userID)

	var k OneTimePreKeyInput
	if err := row.Scan(&k.KeyID, &k.PublicKey); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE one_time_prekeys SET used_at = ? WHERE user_id = ? AND key_id = ?`,
		now, userID, k.KeyID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &k, nil
}
