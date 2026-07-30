package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWorkerPool_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	urls := make([]string, 20)
	for i := range urls {
		urls[i] = server.URL
	}

	jobs := make(chan string, len(urls))
	results := make(chan checkResult, len(urls))
	var wg sync.WaitGroup

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numWorkers := 5
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, &wg, newTestConfig(200*time.Millisecond))
	}

	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()
	close(results)

	successCount := 0
	for r := range results {
		if r.status == statusHealthy {
			successCount++
		}
	}

	if successCount != len(urls) {
		t.Errorf("got %v urls success, want %v urls", successCount, len(urls))
	}
}

func TestWorkerPool_MixedConditions_Race(t *testing.T) {
	var mu sync.Mutex
	attempts := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts[r.URL.Path]++
		count := attempts[r.URL.Path]
		mu.Unlock()

		switch {
		case r.URL.Path == "/ok":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == "/servererror":
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(r.URL.Path, "/retry"):
			if count < 3 {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	var urls []string
	for range 5 {
		urls = append(urls, server.URL+"/ok")
	}
	for range 3 {
		urls = append(urls, server.URL+"/notfound")
	}
	for range 3 {
		urls = append(urls, server.URL+"/servererror")
	}
	for i := range 4 {
		urls = append(urls, fmt.Sprintf("%s/retry/%d", server.URL, i))
	}

	jobs := make(chan string, len(urls))
	results := make(chan checkResult, len(urls))
	var wg sync.WaitGroup

	ctx := t.Context()

	numWorkers := 5
	for i := range numWorkers {
		wg.Add(1)
		go worker(ctx, i, jobs, results, &wg, newTestConfig(500*time.Millisecond))
	}

	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()
	close(results)

	healthy, failure := 0, 0
	for r := range results {
		switch r.status {
		case statusHealthy:
			healthy++
		case statusFailure:
			failure++
		}
	}

	if healthy != 9 {
		t.Errorf("got %d healthy, want 9", healthy)
	}
	if failure != 6 {
		t.Errorf("got %d failure, want 6", failure)
	}

	mu.Lock()
	for i := range 4 {
		path := fmt.Sprintf("/retry/%d", i)
		if attempts[path] != 3 {
			t.Errorf("path %s got %d attempts, want 3", path, attempts[path])
		}
	}
	mu.Unlock()
}
