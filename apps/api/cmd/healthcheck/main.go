package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	client := http.Client{Timeout: 500 * time.Millisecond}
	res, err := client.Get("http://127.0.0.1:8080/ready")
	if err != nil {
		os.Exit(1)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}
