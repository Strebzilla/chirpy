package main

import (
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/Strebzilla/chirpy/internal/database"

	_ "github.com/lib/pq"
)

type apiConfig struct {
	dbQueries      *database.Queries
	platform       string
	fileserverHits atomic.Int32
}

func main() {
	const port = "8080"

	mux := setupServer()
	dbQueries := setupDatabase()

	apiConfig := apiConfig{}
	apiConfig.dbQueries = dbQueries
	apiConfig.platform = os.Getenv("PLATFORM")

	setupHandlers(mux, &apiConfig)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	slog.Info("Serving...", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}
}
