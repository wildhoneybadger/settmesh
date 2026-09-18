package storage

import (
	"context"
	"errors"
	"time"
)

// ErrMessageNotFound est retourné quand le message est inconnu, expiré, ou
// n'appartient pas au destinataire qui tente d'y accéder.
var ErrMessageNotFound = errors.New("message introuvable")

// InboundMessage est un message en attente pour un destinataire, avec le nom
// d'utilisateur de l'expéditeur déjà résolu pour l'affichage côté client.
type InboundMessage struct {
	ID             string
	SenderUsername string
	Ciphertext     []byte
	CreatedAt      time.Time
}

// CreateMessage dépose un message chiffré dans la boîte du destinataire.
func (db *DB) CreateMessage(ctx context.Context, id, senderID, recipientID string, ciphertext []byte, now, expiresAt time.Time) error {
	_, err := db.conn.ExecContext(ctx,
		`INSERT INTO messages (id, sender_id, recipient_id, ciphertext, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, senderID, recipientID, ciphertext, now, expiresAt,
	)
	return err
}

// CountPendingMessages retourne le nombre de messages en attente pour recipientID,
// utilisé pour appliquer le quota par compte.
func (db *DB) CountPendingMessages(ctx context.Context, recipientID string) (int, error) {
	var count int
	err := db.conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM messages WHERE recipient_id = ?`, recipientID,
	).Scan(&count)
	return count, err
}

// ListPendingMessages retourne, du plus ancien au plus récent, jusqu'à limit messages
// en attente pour recipientID. Ne les supprime pas.
func (db *DB) ListPendingMessages(ctx context.Context, recipientID string, limit int) ([]InboundMessage, error) {
	rows, err := db.conn.QueryContext(ctx,
		`SELECT m.id, u.username, m.ciphertext, m.created_at
		 FROM messages m JOIN users u ON u.id = m.sender_id
		 WHERE m.recipient_id = ?
		 ORDER BY m.created_at ASC
		 LIMIT ?`,
		recipientID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []InboundMessage
	for rows.Next() {
		var m InboundMessage
		if err := rows.Scan(&m.ID, &m.SenderUsername, &m.Ciphertext, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// DeleteMessage supprime un message, mais seulement s'il est bien adressé à
// recipientID : un utilisateur ne peut accuser réception que de ses propres messages.
func (db *DB) DeleteMessage(ctx context.Context, id, recipientID string) error {
	res, err := db.conn.ExecContext(ctx,
		`DELETE FROM messages WHERE id = ? AND recipient_id = ?`, id, recipientID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMessageNotFound
	}
	return nil
}

// DeleteExpiredMessages supprime les messages jamais livrés dont la durée de
// rétention a expiré.
func (db *DB) DeleteExpiredMessages(ctx context.Context, before time.Time) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM messages WHERE expires_at < ?`, before)
	return err
}
