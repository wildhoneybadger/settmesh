package handlers

import (
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

// Server regroupe les dépendances partagées par les handlers HTTP authentifiés.
type Server struct {
	DB           *storage.DB
	Files        *storage.BlobStore
	LoginLimiter *auth.LoginLimiter
	// AdminToken protège les routes d'administration. Si vide, ces routes refusent
	// toute requête (fail closed) plutôt que d'être exposées sans protection.
	AdminToken string
	// InviteTTL est la durée de validité des codes d'invitation générés.
	InviteTTL time.Duration
	// SessionTTL est la durée de validité d'un jeton de session émis à la connexion.
	SessionTTL time.Duration
	// MessageTTL est la durée de rétention maximale d'un message non livré.
	MessageTTL time.Duration
	// FileTTL est la durée de rétention maximale d'une photo non livrée.
	FileTTL time.Duration
}
