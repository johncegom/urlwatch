package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func worker(ctx context.Context, id int, jobs <-chan string, results chan<- checkResult, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	for {
		select {
		case url, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Printf("worker %d - picked up %v \n", id, url)
			checkURL(url, results, timeout)

		case <-ctx.Done():
			return
		}
	}
}
