// Package config charge la configuration du serveur depuis les variables d'environnement.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config regroupe les paramètres de configuration du serveur.
type Config struct {
	// Port sur lequel le serveur HTTP écoute.
	Port string
	// DBPath est le chemin du fichier de base de données SQLite.
	DBPath string
	// FilesDir est le dossier où sont stockés les fichiers chiffrés (photos).
	FilesDir string
	// AdminToken protège les routes d'administration (génération des codes
	// d'invitation). Si vide, ces routes refusent toute requête.
	AdminToken string
	// InviteCodeTTL est la durée de validité d'un code d'invitation généré.
	InviteCodeTTL time.Duration
	// SessionTTL est la durée de validité d'un jeton de session émis à la connexion.
	SessionTTL time.Duration
	// MessageTTL est la durée de rétention maximale d'un message texte non livré
	// (30 jours par défaut, voir CLAUDE.md).
	MessageTTL time.Duration
	// FileTTL est la durée de rétention maximale d'une photo non livrée
	// (7 jours par défaut, voir CLAUDE.md).
	FileTTL time.Duration
}

// Load construit la configuration à partir des variables d'environnement,
// avec des valeurs par défaut raisonnables pour le développement local.
func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		DBPath:        getEnv("DB_PATH", "./data/settmesh.db"),
		FilesDir:      getEnv("FILES_DIR", "./data/files"),
		AdminToken:    getEnv("ADMIN_TOKEN", ""),
		InviteCodeTTL: time.Duration(getEnvInt("INVITE_TTL_HOURS", 72)) * time.Hour,
		SessionTTL:    time.Duration(getEnvInt("SESSION_TTL_HOURS", 24*30)) * time.Hour,
		MessageTTL:    time.Duration(getEnvInt("MESSAGE_TTL_HOURS", 24*30)) * time.Hour,
		FileTTL:       time.Duration(getEnvInt("FILE_TTL_HOURS", 24*7)) * time.Hour,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}
