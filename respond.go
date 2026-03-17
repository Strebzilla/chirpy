package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/Strebzilla/chirpy/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

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
