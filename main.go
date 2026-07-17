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
	results := make(chan string, len(urls))
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

	for v := range results {
		fmt.Println(v)
	}
}
