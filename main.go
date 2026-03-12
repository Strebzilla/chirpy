package main

import (
	"log"
	"net/http"
)


func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	const healthCheckBody = "OK"
	
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func main() {
	const filepathroot = "./app"
	const port = "8080"

	mux := http.NewServeMux()
	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir(filepathroot))))
	mux.HandleFunc("/healthz", healthCheckHandler)

	srv := &http.Server{
		Addr: ":8080",
		Handler: mux,
	}
	
	log.Printf("Serving files from %s on port: %s\n", filepathroot, port)
	log.Fatal(srv.ListenAndServe())
}
