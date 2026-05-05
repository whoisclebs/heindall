package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"
)

func main() {
	client, url := clientAndURL()
	res, err := client.Get(url)
	if err != nil {
		os.Exit(1)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}

func clientAndURL() (http.Client, string) {
	if socketPath := os.Getenv("SOCKET_PATH"); socketPath != "" {
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		}
		return http.Client{Timeout: 500 * time.Millisecond, Transport: transport}, "http://unix/ready"
	}
	return http.Client{Timeout: 500 * time.Millisecond}, "http://127.0.0.1:8080/ready"
}
