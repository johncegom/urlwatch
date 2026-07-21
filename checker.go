package main

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type checkStatus string

const (
	statusHealthy   checkStatus = "healthy"
	statusReachable checkStatus = "reachable"
	statusFailure   checkStatus = "failure"
)

type checkResult struct {
	url        string
	status     checkStatus
	statusCode int    // 0 if the request never got a response at all
	errMsg     string // empty if statusCode is set and valid
	latency    time.Duration
	checkedAt  time.Time
}

func newCheckResult(url string, status checkStatus, statusCode int, errMsg string, start time.Time) checkResult {
	return checkResult{
		url:        url,
		status:     status,
		statusCode: statusCode,
		errMsg:     errMsg,
		latency:    time.Since(start),
		checkedAt:  time.Now(),
	}
}

func checkURL(url string, results chan<- checkResult) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		results <- newCheckResult(url, statusFailure, 0, err.Error(), start)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			results <- newCheckResult(url, statusFailure, 0, "timeout", start)
			return
		}
		results <- newCheckResult(url, statusFailure, 0, err.Error(), start)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		results <- newCheckResult(url, statusHealthy, resp.StatusCode, "", start)
		return
	} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
		results <- newCheckResult(url, statusReachable, resp.StatusCode, "may require authentication", start)
		return
	} else {
		results <- newCheckResult(url, statusFailure, resp.StatusCode, "", start)
		return
	}
}
