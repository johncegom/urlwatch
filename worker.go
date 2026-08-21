package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/johncegom/urlwatch/checker"
)

type job struct {
	index int
	url   string
}

func worker(ctx context.Context, id int, jobs <-chan job, results chan<- checker.CheckResult, wg *sync.WaitGroup, cfg checker.CheckerConfig) {
	defer wg.Done()
	for {
		select {
		case j, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Printf("worker %d - picked up %v \n", id, j.url)
			result := checker.CheckURL(j.url, j.index, cfg)
			results <- result

		case <-ctx.Done():
			return
		}
	}
}
