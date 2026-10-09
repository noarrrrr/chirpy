package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/noarrrrr/chirpy/internal/database"
)

type apiConfig struct {
	dbQueries      *database.Queries
	fileserverHits atomic.Int32
	platform       string
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("godotenv: %v", err)
	}
	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	dbQueries := database.New(db)

	const filepathRoot = "."
	const port = "8080"
	cfg := apiConfig{
		dbQueries:      dbQueries,
		fileserverHits: atomic.Int32{},
		platform:       os.Getenv("PLATFORM"),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", healthzHandler)

	mux.HandleFunc("GET /admin/metrics", cfg.metricsHandler)

	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)

	mux.HandleFunc("POST /api/chirps", cfg.chirpHandler)

	mux.HandleFunc("GET /api/chirps", cfg.chirpsRetrieveHandler)

	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.chirpRetrieveHandler)

	mux.HandleFunc("POST /api/users", cfg.createUserHandler)

	mux.HandleFunc("POST /api/login", cfg.loginUserHandler)

	mux.Handle("/app/", http.StripPrefix("/app", cfg.incrementHits(http.FileServer(http.Dir(filepathRoot)))))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepathRoot, port)
	log.Fatal(srv.ListenAndServe())
}
