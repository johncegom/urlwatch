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

func classify(statusCode int) checkStatus {
	if statusCode >= 200 && statusCode < 300 {
		return statusHealthy
	} else if statusCode == 401 || statusCode == 403 {
		return statusReachable
	} else {
		return statusFailure
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

	status := classify(resp.StatusCode)
	errMsg := ""
	if status == statusReachable {
		errMsg = "may require authentication"
	}

	results <- newCheckResult(url, status, resp.StatusCode, errMsg, start)
}
