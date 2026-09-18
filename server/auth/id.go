package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID génère un identifiant aléatoire unique (128 bits, encodé en hexadécimal).
func GenerateID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
