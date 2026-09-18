package auth

import (
	"crypto/rand"
	"strings"
)

// inviteCodeAlphabet exclut les caractères ambigus (0/O, 1/I/L) pour la lisibilité.
// Sa longueur (32) divise 256 exactement, donc l'échantillonnage par octet aléatoire
// ci-dessous ne produit aucun biais statistique.
const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateInviteCode génère un code d'invitation aléatoire et imprévisible (80 bits
// d'entropie), formaté en groupes de 4 caractères pour la lisibilité.
func GenerateInviteCode() (string, error) {
	const length = 16

	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	var sb strings.Builder
	for i, b := range raw {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(inviteCodeAlphabet[int(b)%len(inviteCodeAlphabet)])
	}
	return sb.String(), nil
}
