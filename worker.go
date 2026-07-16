package main

import (
	"context"
	"fmt"
	"sync"
)

func worker(ctx context.Context, id int, jobs <-chan string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case url, ok := <-jobs:
			if !ok {
				return
			}
			fmt.Printf("worker %d - picked up %v \n", id, url)
			checkURL(url, results)

		case <-ctx.Done():
			return
		}
	}
}
