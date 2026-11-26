package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	client := http.Client{Timeout: 2 * time.Second}
	target := os.Getenv("HEALTHCHECK_URL")
	if target == "" {
		target = "http://127.0.0.1:8080/health/ready"
	}
	response, err := client.Get(target)
	if err != nil {
		os.Exit(1)
	}
	_ = response.Body.Close()
	if response.StatusCode != 200 {
		os.Exit(1)
	}
}
