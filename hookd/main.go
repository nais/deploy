package main

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func serve(w http.ResponseWriter, r *http.Request) {
	log.Debug(r)
	w.WriteHeader(200)
}

func main() {
	http.HandleFunc("/", serve)
	server := &http.Server{
		Addr: ":8080",
	}
	server.ListenAndServe()
}
