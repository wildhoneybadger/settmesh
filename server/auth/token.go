package auth

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSessionToken génère un jeton de session opaque et imprévisible (256 bits).
func GenerateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
