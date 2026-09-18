package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrFileNotFound est retourné quand le fichier est inconnu, ou n'appartient pas au
// destinataire qui tente d'y accéder.
var ErrFileNotFound = errors.New("fichier introuvable")

// FileMeta décrit une photo chiffrée en attente de livraison. Le contenu chiffré
// lui-même est stocké à part (voir BlobStore) ; cette table n'en garde que les
// métadonnées.
type FileMeta struct {
	ID          string
	SenderID    string
	RecipientID string
	SizeBytes   int64
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// CreateFile enregistre les métadonnées d'un fichier chiffré déjà écrit sur le disque.
func (db *DB) CreateFile(ctx context.Context, id, senderID, recipientID string, sizeBytes int64, now, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO files (id, sender_id, recipient_id, size_bytes, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, senderID, recipientID, sizeBytes, now, expiresAt,
	)
	return err
}

// GetFile récupère les métadonnées d'un fichier par son identifiant.
func (db *DB) GetFile(ctx context.Context, id string) (*FileMeta, error) {
	row := db.conn.QueryRowContext(ctx,
		`SELECT id, sender_id, recipient_id, size_bytes, created_at, expires_at FROM files WHERE id = ?`, id)

	var f FileMeta
	if err := row.Scan(&f.ID, &f.SenderID, &f.RecipientID, &f.SizeBytes, &f.CreatedAt, &f.ExpiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	return &f, nil
}

// SumPendingFileBytes retourne le volume total (en octets) de fichiers en attente
// pour recipientID, utilisé pour appliquer le quota de stockage par compte.
func (db *DB) SumPendingFileBytes(ctx context.Context, recipientID string) (int64, error) {
	var total int64
	err := db.conn.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size_bytes), 0) FROM files WHERE recipient_id = ?`, recipientID,
	).Scan(&total)
	return total, err
}

// DeleteFile supprime les métadonnées d'un fichier, mais seulement s'il est bien
// adressé à recipientID.
func (db *DB) DeleteFile(ctx context.Context, id, recipientID string) error {
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM files WHERE id = ? AND recipient_id = ?`, id, recipientID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrFileNotFound
	}
	return nil
}

// ListExpiredFileIDs retourne les identifiants des fichiers dont la rétention a
// expiré, pour purger leurs octets sur disque avant de supprimer leurs métadonnées.
func (db *DB) ListExpiredFileIDs(ctx context.Context, before time.Time) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT id FROM files WHERE expires_at < ?`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteExpiredFileRecords supprime les métadonnées des fichiers expirés. À appeler
// après avoir purgé leurs octets sur disque (voir ListExpiredFileIDs).
func (db *DB) DeleteExpiredFileRecords(ctx context.Context, before time.Time) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM files WHERE expires_at < ?`, before)
	return err
}
