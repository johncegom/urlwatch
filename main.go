package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync"
	"time"
)

func randomNumberRange(min int, max int) int {
	rangeInt := min + rand.IntN(max-min+1)
	return rangeInt
}

func checkURL(url string, results chan<- string) {
	delay := randomNumberRange(200, 1000)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	select {
	case <-time.After(time.Duration(delay) * time.Millisecond):
		// TODO: work finished normally — send the "is up" result, same as before
		results <- fmt.Sprint(url, " is up")
	case <-ctx.Done():
		// TODO: context timed out or was cancelled first — send a result saying so
		// hint: ctx.Err() tells you why (e.g. "context deadline exceeded")
		results <- fmt.Sprintf("%s: %v", url, ctx.Err())
	}
}

func worker(ctx context.Context, id int, jobs <-chan string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case url, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Println("worker", id, "picked up ", url)
			checkURL(url, results)

		case <-ctx.Done():
			return
		}
	}
}

func main() {
	urls := []string{
		"https://example.com",
		"https://another.com",
		"https://a-third.com",
		"https://google.com",
		"https://github.com",
		"https://golang.org",
		"https://wikipedia.org",
		"https://reddit.com",
		"https://twitter.com",
		"https://youtube.com",
		"https://amazon.com",
		"https://facebook.com",
		"https://instagram.com",
		"https://linkedin.com",
		"https://microsoft.com",
	}
	jobs := make(chan string, len(urls))
	results := make(chan string, len(urls))
	var wg sync.WaitGroup

	numWorkers := 6

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, &wg)
	}

	for _, u := range urls {
		jobs <- u
	}
	close(jobs)

	wg.Wait()
	close(results)

	for v := range results {
		fmt.Println(v)
	}
}
