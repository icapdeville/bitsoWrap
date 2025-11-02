package main

import (
    "log"
    "net/http"
    "bitsoWrap/internal/api"
)

func main() {
    router := api.NewRouter()

    log.Println("Servidor escuchando en :8080")
    http.ListenAndServe(":8080", router)
}
