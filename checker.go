package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

func checkURL(url string, results chan<- string) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		results <- fmt.Sprintf("%v failed with error: %v", url, err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			results <- fmt.Sprintf("%v failed because of timeout", url)
			return
		}
		results <- fmt.Sprintf("%v failed with error: %v", url, err)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	results <- fmt.Sprintf("%v - %d - %v", url, resp.StatusCode, latency.Round(time.Millisecond))
}
