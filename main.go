package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/sebradloff/monday-gcal-integration/handlers"
)

func main() {
	server := &http.Server{
		Addr:    fmt.Sprintf(":8080"),
		Handler: handlers.New(),
	}

	log.Printf("Starting HTTP Server. Listening at %q", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Printf("%v", err)
	} else {
		log.Println("Server closed!")
	}
}
