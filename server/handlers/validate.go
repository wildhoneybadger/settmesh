package handlers

import "encoding/base64"

// Tailles imposées par le protocole Signal : clés publiques Curve25519 (32 octets) et
// signatures XEdDSA (64 octets), encodées en base64 standard par le client.
const (
	curve25519KeyLen = 32
	signatureLen     = 64
)

func decodeFixedLenKey(s string, length int) bool {
	b, err := base64.StdEncoding.DecodeString(s)
	return err == nil && len(b) == length
}
