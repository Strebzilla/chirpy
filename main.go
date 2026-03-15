package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/Strebzilla/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func (cfg *apiConfig) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	responseText := fmt.Sprintf(`
	<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
	</html>
	`, cfg.fileserverHits.Load())

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(responseText))
	if err != nil {
		slog.Error("Error sending response", "error", err, "operation", "http.ResponseWriter.Write")
	}
}

func (cfg *apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(http.StatusText(http.StatusOK)))
	if err != nil {
		slog.Error("Error sending response", "error", err, "operation", "http.ResponseWriter.Write")
	}
}

func censorProfaneWords(s string) string {
	original := strings.Split(s, " ")

	censoredWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}
	censor := "****"

	s = strings.ToLower(s)
	splitCensoredString := strings.Split(s, " ")

	for i, userWord := range splitCensoredString {
		for _, censoredWord := range censoredWords {
			if userWord == censoredWord {
				original[i] = censor
			}
		}
	}
	censoredString := strings.Join(original, " ")
	return censoredString
}

func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Body string `json:"body"`
	}
	type responseBody struct {
		Valid        bool   `json:"valid"`
		Cleaned_body string `json:"cleaned_body"`
	}

	requestData, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't read request")
		return
	}
	request := requestBody{}
	err = json.Unmarshal(requestData, &request)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	if len(request.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}
	censoredString := censorProfaneWords(request.Body)
	respondWithJSON(w, http.StatusOK, responseBody{
		Valid:        true,
		Cleaned_body: censoredString,
	})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Error marshaling json", "error", err, "operation", "json.Marshal")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`{"error":"Internal Server Error"}`))
		if err != nil {
			slog.Error("Error sending response", "error", err, "operation", "http.ResponseWriter.Write")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(response)
	if err != nil {
		slog.Error("Error sending response", "error", err, "operation", "http.ResponseWriter.Write")
		return
	}
}

func setupServer() *http.ServeMux {
	err := godotenv.Load()
	if err != nil {
		slog.Error("Error reading environment variables")
		os.Exit(1)
	}

	return http.NewServeMux()
}

func setupDatabase() *database.Queries {
	dbURL := os.Getenv("DB_URL")
	dbConnection, err := sql.Open("postgres", dbURL)
	if err != nil {
		slog.Error("Error connecting to postgres database")
		os.Exit(1)
	}

	dbQueries := database.New(dbConnection)

	return dbQueries
}

func setupHandlers(mux *http.ServeMux, cfg *apiConfig) {
	const filePathRoot = "./app"

	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(filePathRoot)))
	mux.Handle("/app/", cfg.middlewareMetricsInc(appHandler))
	mux.HandleFunc("GET /admin/metrics", cfg.getMetricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetMetricsHandler)
	mux.HandleFunc("GET /api/healthz", healthCheckHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
}

func main() {
	const port = "8080"

	mux := setupServer()
	dbQueries := setupDatabase()

	apiConfig := apiConfig{}
	apiConfig.dbQueries = dbQueries

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
