package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg * apiConfig) getMetricsHandler(w http.ResponseWriter, r *http.Request) {
	responseText := fmt.Sprintf("Hits: %d", cfg.fileserverHits.Load())
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(responseText))
}

func (cfg * apiConfig) resetMetricsHandler(w http.ResponseWriter, r *http.Request) {
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
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func main() {
	const filepathroot = "./app"
	const port = "8080"

	apiConfig := apiConfig{}

	mux := http.NewServeMux()
	appHandler := http.StripPrefix("/app", http.FileServer(http.Dir(filepathroot)))

	mux.Handle("/app/", apiConfig.middlewareMetricsInc(appHandler))
	mux.HandleFunc("/healthz", healthCheckHandler)
	mux.HandleFunc("/metrics", apiConfig.getMetricsHandler)
	mux.HandleFunc("/reset", apiConfig.resetMetricsHandler)

	srv := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	
	log.Printf("Serving files from %s on port: %s\n", filepathroot, port)
	log.Fatal(srv.ListenAndServe())
}
