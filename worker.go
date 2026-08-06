package main

import (
	"context"
	"fmt"
	"sync"
)

type job struct {
	index int
	url   string
}

func worker(ctx context.Context, id int, jobs <-chan job, results chan<- checkResult, wg *sync.WaitGroup, cfg checkerConfig) {
	defer wg.Done()
	for {
		select {
		case j, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Printf("worker %d - picked up %v \n", id, j.url)
			checkURL(j, results, cfg)

		case <-ctx.Done():
			return
		}
	}
}
