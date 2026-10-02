package main

import (
	"bitsoWrap/internal/api"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := api.NewRouter()

	log.Printf("Servidor escuchando en :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
