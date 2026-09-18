// Package storage gère la persistance des données du serveur (utilisateurs, codes d'invitation).
package storage

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB encapsule la connexion à la base de données SQLite.
type DB struct {
	conn *sql.DB
}

// Open ouvre (ou crée) la base SQLite au chemin donné et applique les migrations.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("ouverture base de données: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("connexion base de données: %w", err)
	}

	// SQLite ne supporte pas bien les écritures concurrentes multiples ; une seule
	// connexion évite les erreurs "database is locked" à ce stade du projet.
	conn.SetMaxOpenConns(1)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migration base de données: %w", err)
	}

	return db, nil
}

// Close ferme la connexion à la base de données.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	const schema = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS invite_codes (
		code TEXT PRIMARY KEY,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL,
		used_at DATETIME,
		used_by TEXT REFERENCES users(id)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id TEXT NOT NULL REFERENCES users(id),
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);

	-- Bundle de clés publiques Signal (X3DH) par utilisateur. Le serveur ne fait que
	-- stocker/relayer ces clés en aveugle, sans jamais voir la moindre clé privée.
	CREATE TABLE IF NOT EXISTS identity_keys (
		user_id TEXT PRIMARY KEY REFERENCES users(id),
		public_key TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS signed_prekeys (
		user_id TEXT PRIMARY KEY REFERENCES users(id),
		key_id INTEGER NOT NULL,
		public_key TEXT NOT NULL,
		signature TEXT NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS one_time_prekeys (
		user_id TEXT NOT NULL REFERENCES users(id),
		key_id INTEGER NOT NULL,
		public_key TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		used_at DATETIME,
		PRIMARY KEY (user_id, key_id)
	);

	-- Messages chiffrés de bout en bout, en attente de livraison. Le serveur ne voit
	-- jamais le contenu en clair ; ciphertext est un blob opaque. Un message est
	-- supprimé dès que le destinataire en accuse réception, ou automatiquement à
	-- expiration (30 jours maximum, voir CLAUDE.md) s'il n'a jamais été livré.
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		sender_id TEXT NOT NULL REFERENCES users(id),
		recipient_id TEXT NOT NULL REFERENCES users(id),
		ciphertext BLOB NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_messages_recipient ON messages(recipient_id, created_at);

	-- Photos chiffrées. Les octets chiffrés sont stockés à part sur le disque (voir
	-- BlobStore), séparément du texte ; cette table n'en garde que les métadonnées
	-- minimales. La clé de déchiffrement symétrique n'y transite jamais : elle est
	-- envoyée chiffrée de bout en bout comme un message classique.
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		sender_id TEXT NOT NULL REFERENCES users(id),
		recipient_id TEXT NOT NULL REFERENCES users(id),
		size_bytes INTEGER NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_files_recipient ON files(recipient_id, created_at);
	`
	_, err := db.conn.Exec(schema)
	return err
}
