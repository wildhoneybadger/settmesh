package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// BlobStore stocke sur disque le contenu chiffré des fichiers (photos), séparément
// de la base de données qui n'en garde que les métadonnées (voir files.go). Le
// serveur ne fait que manipuler des octets opaques : il ne les déchiffre jamais.
type BlobStore struct {
	dir string
}

// NewBlobStore crée (si besoin) le dossier dir et retourne un BlobStore qui y stocke
// ses fichiers.
func NewBlobStore(dir string) (*BlobStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("création du dossier de fichiers: %w", err)
	}
	return &BlobStore{dir: dir}, nil
}

func (b *BlobStore) path(id string) string {
	return filepath.Join(b.dir, id)
}

// Write écrit le contenu de r sur le disque sous l'identifiant id, et retourne le
// nombre d'octets écrits. En cas d'erreur (y compris si r dépasse une limite de
// taille imposée par l'appelant), le fichier partiel est supprimé.
func (b *BlobStore) Write(id string, r io.Reader) (int64, error) {
	f, err := os.OpenFile(b.path(id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		os.Remove(b.path(id))
		return 0, err
	}
	return n, nil
}

// Open ouvre un fichier stocké pour lecture.
func (b *BlobStore) Open(id string) (*os.File, error) {
	return os.Open(b.path(id))
}

// Delete supprime un fichier stocké. Ignore silencieusement son absence.
func (b *BlobStore) Delete(id string) error {
	err := os.Remove(b.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
