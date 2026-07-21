package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
)

func main() {
	filePath := parseFlags()

	urls, warnings, err := readURLs(filePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, w := range warnings {
		fmt.Println("warning: ", w)
	}

	jobs := make(chan string, len(urls))
	results := make(chan checkResult, len(urls))
	var wg sync.WaitGroup
	numWorkers := 5

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

	hasFailure := false

	for r := range results {
		switch r.status {
		case statusHealthy:
			fmt.Printf("%v is %v(%v) \n", r.url, r.status, r.statusCode)
		case statusReachable:
			fmt.Printf("%v is %v(%v) but %v \n", r.url, r.status, r.statusCode, r.errMsg)
		default:
			fmt.Printf("%v is %v(%v) with error: %v \n", r.url, r.status, r.statusCode, r.errMsg)
			hasFailure = true
		}
	}

	if hasFailure {
		os.Exit(1)
	}
}
