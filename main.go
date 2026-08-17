package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"sync"
)

func processResults(w io.Writer, results []checkResult, jsonOutput bool) bool {
	hasFailure := false

	for _, r := range results {
		if r.Status == statusFailure {
			hasFailure = true
			break
		}
	}

	if jsonOutput {
		slices.SortFunc(results, func(a, b checkResult) int {
			return cmp.Compare(a.Index, b.Index)
		})
		data, err := json.Marshal(results)
		if err != nil {
			fmt.Fprintln(w, err)
			os.Exit(1)
		}

		fmt.Fprintln(w, string(data))
	} else {
		for _, r := range results {
			switch r.Status {
			case statusHealthy:
				fmt.Fprintf(w, "%v is %v(%v) \n", r.Url, r.Status, r.StatusCode)
			case statusReachable:
				fmt.Fprintf(w, "%v is %v(%v) but %v \n", r.Url, r.Status, r.StatusCode, r.ErrMsg)
			default:
				fmt.Fprintf(w, "%v is %v(%v) with error: %v \n", r.Url, r.Status, r.StatusCode, r.ErrMsg)
			}
		}

	}
	return hasFailure
}

func main() {
	flagConfigs := parseFlags()

	urls, warnings, err := readURLs(flagConfigs.filePath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, w := range warnings {
		fmt.Println("warning: ", w)
	}

	jobs := make(chan job, len(urls))
	results := make(chan checkResult, len(urls))
	var wg sync.WaitGroup
	numWorkers := flagConfigs.numWorkers

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := newProductionConfig(flagConfigs.reqTimeout)
	for i := 1; i <= numWorkers; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, &wg, cfg)
	}

	for i, u := range urls {
		jobs <- job{
			index: i,
			url:   u,
		}
	}
	close(jobs)

	wg.Wait()
	close(results)

	var allResults []checkResult
	for r := range results {
		allResults = append(allResults, r)
	}

	hasFailure := processResults(os.Stdout, allResults, flagConfigs.jsonOutput)

	if hasFailure {
		os.Exit(1)
	}
}
