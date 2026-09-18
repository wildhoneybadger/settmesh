package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/wildhoneybadger/settmesh/server/auth"
	"github.com/wildhoneybadger/settmesh/server/config"
	"github.com/wildhoneybadger/settmesh/server/handlers"
	"github.com/wildhoneybadger/settmesh/server/storage"
)

func main() {
	cfg := config.Load()

	if dir := filepath.Dir(cfg.DBPath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Fatalf("création du dossier de données: %v", err)
		}
	}

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("ouverture de la base de données: %v", err)
	}
	defer db.Close()

	files, err := storage.NewBlobStore(cfg.FilesDir)
	if err != nil {
		log.Fatalf("ouverture du stockage de fichiers: %v", err)
	}

	if cfg.AdminToken == "" {
		log.Println("ATTENTION: ADMIN_TOKEN non défini, les routes d'administration sont désactivées")
	}

	limiter := auth.NewLoginLimiter()
	go cleanupLoop(limiter, 10*time.Minute)
	go sessionCleanupLoop(db, time.Hour)
	go messageCleanupLoop(db, time.Hour)
	go fileCleanupLoop(db, files, time.Hour)

	srv := &handlers.Server{
		DB:           db,
		Files:        files,
		LoginLimiter: limiter,
		AdminToken:   cfg.AdminToken,
		InviteTTL:    cfg.InviteCodeTTL,
		SessionTTL:   cfg.SessionTTL,
		MessageTTL:   cfg.MessageTTL,
		FileTTL:      cfg.FileTTL,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("POST /auth/register", srv.Register)
	mux.HandleFunc("POST /auth/login", srv.Login)
	mux.HandleFunc("POST /admin/invite-codes", srv.CreateInviteCode)

	mux.HandleFunc("PUT /keys/identity", srv.RequireAuth(srv.UploadIdentityKey))
	mux.HandleFunc("PUT /keys/signed-prekey", srv.RequireAuth(srv.UploadSignedPreKey))
	mux.HandleFunc("POST /keys/one-time-prekeys", srv.RequireAuth(srv.UploadOneTimePreKeys))
	mux.HandleFunc("GET /keys/one-time-prekeys/count", srv.RequireAuth(srv.OneTimePreKeyCount))
	mux.HandleFunc("GET /keys/bundle/{username}", srv.RequireAuth(srv.KeyBundle))

	mux.HandleFunc("POST /messages", srv.RequireAuth(srv.SendMessage))
	mux.HandleFunc("GET /messages", srv.RequireAuth(srv.ListMessages))
	mux.HandleFunc("DELETE /messages/{id}", srv.RequireAuth(srv.AckMessage))

	mux.HandleFunc("POST /files", srv.RequireAuth(srv.UploadFile))
	mux.HandleFunc("GET /files/{id}", srv.RequireAuth(srv.DownloadFile))
	mux.HandleFunc("DELETE /files/{id}", srv.RequireAuth(srv.AckFile))

	addr := ":" + cfg.Port
	log.Printf("settmesh server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func cleanupLoop(limiter *auth.LoginLimiter, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		limiter.Cleanup(time.Hour)
	}
}

func sessionCleanupLoop(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := db.DeleteExpiredSessions(context.Background(), time.Now().UTC()); err != nil {
			log.Printf("nettoyage des sessions expirées: %v", err)
		}
	}
}

func messageCleanupLoop(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := db.DeleteExpiredMessages(context.Background(), time.Now().UTC()); err != nil {
			log.Printf("nettoyage des messages expirés: %v", err)
		}
	}
}

func fileCleanupLoop(db *storage.DB, files *storage.BlobStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UTC()

		ids, err := db.ListExpiredFileIDs(context.Background(), now)
		if err != nil {
			log.Printf("listage des fichiers expirés: %v", err)
			continue
		}
		for _, id := range ids {
			if err := files.Delete(id); err != nil {
				log.Printf("suppression du fichier expiré %s: %v", id, err)
			}
		}

		if err := db.DeleteExpiredFileRecords(context.Background(), now); err != nil {
			log.Printf("nettoyage des métadonnées de fichiers expirés: %v", err)
		}
	}
}
