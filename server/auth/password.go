// Package auth regroupe la logique d'authentification : hachage des mots de passe,
// génération des codes d'invitation et des identifiants, limitation des tentatives.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword dérive un hash bcrypt à partir d'un mot de passe en clair.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword compare un mot de passe en clair à un hash bcrypt.
func VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
