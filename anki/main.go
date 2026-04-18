package main

import (
	"database/sql"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"
)

//go:embed static
var staticFiles embed.FS

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "anki.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1) // SQLite is single-writer
	if err := initDB(db); err != nil {
		log.Fatalf("init db: %v", err)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("GET /api/decks", handleListDecks(db))
	mux.HandleFunc("POST /api/decks", handleCreateDeck(db))
	mux.HandleFunc("DELETE /api/decks/{id}", handleDeleteDeck(db))
	mux.HandleFunc("GET /api/decks/{id}/cards", handleListCards(db))
	mux.HandleFunc("POST /api/decks/{id}/cards", handleCreateCard(db))
	mux.HandleFunc("DELETE /api/cards/{id}", handleDeleteCard(db))
	mux.HandleFunc("GET /api/decks/{id}/study", handleStudy(db))
	mux.HandleFunc("POST /api/cards/{id}/review", handleReview(db))
	mux.HandleFunc("GET /api/decks/{id}/stats", handleDeckStats(db))

	// Static files
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatalf("static fs: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Printf("Listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
