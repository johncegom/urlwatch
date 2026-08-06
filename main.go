package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sync"
)

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

	hasFailure := false

	var handleResult func(checkResult)
	var finish func()

	if flagConfigs.jsonOutput {
		var allResults []checkResult
		handleResult = func(r checkResult) {
			allResults = append(allResults, r)
		}
		finish = func() {
			slices.SortFunc(allResults, func(a, b checkResult) int {
				return cmp.Compare(a.Index, b.Index)

			})
			data, err := json.Marshal(allResults)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

			fmt.Println(string(data))
		}
	} else {
		handleResult = func(r checkResult) {
			switch r.Status {
			case statusHealthy:
				fmt.Printf("%v is %v(%v) \n", r.Url, r.Status, r.StatusCode)
			case statusReachable:
				fmt.Printf("%v is %v(%v) but %v \n", r.Url, r.Status, r.StatusCode, r.ErrMsg)
			default:
				fmt.Printf("%v is %v(%v) with error: %v \n", r.Url, r.Status, r.StatusCode, r.ErrMsg)
			}
		}
		finish = func() {}
	}

	for r := range results {
		if r.Status == statusFailure {
			hasFailure = true
		}
		handleResult(r)
	}
	finish()

	if hasFailure {
		os.Exit(1)
	}
}
