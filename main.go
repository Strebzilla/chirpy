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
	"time"

	"github.com/Strebzilla/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	platform       string
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cfg.fileserverHits.Store(0)
	_, err := cfg.dbQueries.DeleteAllUsers(r.Context())
	if err != nil {
		slog.Error("Error reseting users table", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
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

	body, ok := decodeRequestBody[requestBody](w, r)
	if !ok {
		return
	}

	if len(body.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp is too long")
		return
	}
	censoredString := censorProfaneWords(body.Body)
	respondWithJSON(w, http.StatusOK, responseBody{
		Valid:        true,
		Cleaned_body: censoredString,
	})
}

func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email string `json:"email"`
	}
	type responseBody struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}

	body, ok := decodeRequestBody[requestBody](w, r)
	if !ok {
		return
	}

	user, err := cfg.dbQueries.CreateUser(r.Context(), body.Email)
	if err != nil {
		slog.Error("Error creating user", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	response := responseBody{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	}
	respondWithJSON(w, http.StatusCreated, response)
}

func decodeRequestBody[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var result T
	data, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't read request")
		return result, false
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return result, false
	}
	return result, true
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
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)
	mux.HandleFunc("GET /api/healthz", healthCheckHandler)
	mux.HandleFunc("POST /api/users", cfg.usersHandler)
	mux.HandleFunc("POST /api/validate_chirp", validateChirpHandler)
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
