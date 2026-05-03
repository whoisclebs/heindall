package main

import (
	"log"

	"github.com/whoisclebs/heindall/apps/api/internal/app"
)

func main() {
	server, err := app.NewServer(app.LoadConfig())
	if err != nil {
		log.Fatal(err)
	}
	server.Listen()
}
