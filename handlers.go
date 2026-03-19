package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Strebzilla/chirpy/internal/auth"
	"github.com/Strebzilla/chirpy/internal/database"
	"github.com/google/uuid"
)

type chirpsResponse struct {
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
}

type usersResponse struct {
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	ID          uuid.UUID `json:"id"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

var accessTokenTimeout time.Duration = time.Hour

func setupHandlers(mux *http.ServeMux, cfg *apiConfig) {
	const filePathRoot = "./app"

	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(filePathRoot)))
	mux.Handle("/app/", cfg.middlewareMetricsInc(appHandler))
	mux.HandleFunc("GET /admin/metrics", cfg.getMetricsHandler)
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)

	mux.HandleFunc("POST /api/users", cfg.createUserHandler)
	mux.HandleFunc("PUT /api/users", cfg.updateUserHandler)

	mux.HandleFunc("POST /api/chirps", cfg.chirpsHandler)
	mux.HandleFunc("GET /api/chirps", cfg.getAllChirpsHandler)
	mux.HandleFunc("GET /api/chirps/{id}", cfg.getChirpHandler)
	mux.HandleFunc("DELETE /api/chirps/{id}", cfg.deleteChirpHandler)

	mux.HandleFunc("GET /api/healthz", healthCheckHandler)
	mux.HandleFunc("POST /api/login", cfg.loginHandler)
	mux.HandleFunc("POST /api/refresh", cfg.refreshHandler)
	mux.HandleFunc("POST /api/revoke", cfg.revokeHandler)
	mux.HandleFunc("POST /api/polka/webhooks", cfg.polkaWebhooksHandler)
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func validateChirp(chirp string) error {
	if len(chirp) > 140 {
		return errors.New("chirp is too long")
	}
	return nil
}

func (cfg *apiConfig) authenticateRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	accessToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return uuid.Nil, errors.New("authorization header invalid")
	}
	user_id, err := auth.ValidateJWT(accessToken, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		return uuid.Nil, errors.New("unable to validate jwt token")
	}
	return user_id, nil
}

func respondWithDatabaseError(w http.ResponseWriter, err error) {
	slog.Error("database failure", "error", err)
	respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
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
		respondWithDatabaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(http.StatusText(http.StatusOK)))
	if err != nil {
		slog.Error("Error sending response", "error", err, "operation", "http.ResponseWriter.Write")
	}
}

func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	body, ok := decodeRequestBody[request](w, r)
	if !ok {
		return
	}

	hashedPassword, err := auth.HashPassword(body.Password)
	if err != nil {
		slog.Error("Error hashing password", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	user, err := cfg.dbQueries.CreateUser(r.Context(), database.CreateUserParams{
		Email:          body.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, usersResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

func (cfg *apiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	user_id, err := cfg.authenticateRequest(w, r)
	if err != nil {
		return
	}

	body, ok := decodeRequestBody[request](w, r)
	if !ok {
		return
	}

	hashedPassword, err := auth.HashPassword(body.Password)
	if err != nil {
		slog.Error("Error hashing password", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	user, err := cfg.dbQueries.UpdateUserEmailAndPassword(r.Context(), database.UpdateUserEmailAndPasswordParams{
		ID:             user_id,
		Email:          body.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, usersResponse{
		ID:          user.ID,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Body string `json:"body"`
	}

	user_id, err := cfg.authenticateRequest(w, r)
	if err != nil {
		return
	}

	body, ok := decodeRequestBody[request](w, r)
	if !ok {
		return
	}

	err = validateChirp(body.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	chirp, err := cfg.dbQueries.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   body.Body,
		UserID: user_id,
	})
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}

	respondWithJSON(w, http.StatusCreated, chirpsResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}

	var response []chirpsResponse

	for _, chirp := range chirps {
		response = append(response, chirpsResponse{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (cfg *apiConfig) getChirpHandler(w http.ResponseWriter, r *http.Request) {
	chirpID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), chirpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
			return
		}
		slog.Error("Error getting chirp", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	respondWithJSON(w, http.StatusOK, chirpsResponse{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
	user_id, err := cfg.authenticateRequest(w, r)
	if err != nil {
		return
	}

	chirpID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}

	chirp, err := cfg.dbQueries.GetChirp(r.Context(), chirpID)
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}
	if chirp.UserID != user_id {
		respondWithError(w, http.StatusForbidden, http.StatusText(http.StatusForbidden))
		return
	}

	err = cfg.dbQueries.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	type response struct {
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		ID           uuid.UUID `json:"id"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}

	body, ok := decodeRequestBody[request](w, r)
	if !ok {
		return
	}

	user, err := cfg.dbQueries.GetUser(r.Context(), body.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		respondWithDatabaseError(w, err)
		return
	}

	passwordMatches, err := auth.CheckPasswordHash(body.Password, user.HashedPassword)
	if err != nil {
		slog.Error("Error checking hash", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}
	if !passwordMatches {
		respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	// Create access token
	accessToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, accessTokenTimeout)
	if err != nil {
		slog.Error("Could not make JWT", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	// Create refresh token
	refreshToken := auth.MakeRefreshToken()
	day := 24 * time.Hour
	refreshTokenTimeout := time.Now().Add(day * 60)
	_, err = cfg.dbQueries.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		UserID:    user.ID,
		ExpiresAt: refreshTokenTimeout,
	})
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed,
	})
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Token string `json:"token"`
	}

	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		slog.Error("could not find bearer token in header")
		respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	refreshToken, err := cfg.dbQueries.GetRefreshToken(r.Context(), bearerToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Error("refresh token not found")
			respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		respondWithDatabaseError(w, err)
		return
	}
	now := time.Now()

	if now.After(refreshToken.ExpiresAt) {
		slog.Error("refresh token is expired")
		respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}
	if refreshToken.RevokedAt.Valid {
		slog.Error("refresh token is revoked")
		respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
		return
	}

	// Create access token
	accessToken, err := auth.MakeJWT(refreshToken.UserID, cfg.jwtSecret, accessTokenTimeout)
	if err != nil {
		slog.Error("Could not make JWT", "error", err)
		respondWithError(w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
		return
	}

	respondWithJSON(w, http.StatusOK, response{
		Token: accessToken,
	})
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
		return
	}
	_, err = cfg.dbQueries.ExpireRefreshToken(r.Context(), bearerToken)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
			return
		}
		respondWithDatabaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) polkaWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}

	body, ok := decodeRequestBody[request](w, r)
	if !ok {
		return
	}

	if body.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rows, err := cfg.dbQueries.SetIsChirpyRedWithId(r.Context(), database.SetIsChirpyRedWithIdParams{
		ID:          body.Data.UserID,
		IsChirpyRed: true,
	})
	if err != nil {
		respondWithDatabaseError(w, err)
		return
	}
	if rows == 0 {
		respondWithError(w, http.StatusNotFound, http.StatusText(http.StatusNotFound))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
