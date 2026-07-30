package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"code.dny.dev/ssrf"
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

type checkerConfig struct {
	timeout time.Duration
	client  *http.Client
}

func newProductionConfig(timeout time.Duration) checkerConfig {
	return checkerConfig{
		timeout: timeout,
		client:  safeClient,
	}
}

var safeClient = newSafeClient()

func newSafeClient() *http.Client {
	guardian := ssrf.New()
	dialer := &net.Dialer{Control: guardian.Safe}
	transport := &http.Transport{DialContext: dialer.DialContext}
	return &http.Client{Transport: transport}
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

func doOneAttempt(url string, cfg checkerConfig) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	return cfg.client.Do(req)
}

func checkURL(url string, results chan<- checkResult, cfg checkerConfig) {
	start := time.Now()

	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := doOneAttempt(url, cfg)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				results <- newCheckResult(url, statusFailure, 0, "timeout", start)
				return
			}
			results <- newCheckResult(url, statusFailure, 0, err.Error(), start)
			return
		}

		status := classify(resp.StatusCode)
		errMsg := ""

		if status == statusReachable {
			errMsg = "may require authentication"
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			results <- newCheckResult(url, status, resp.StatusCode, errMsg, start)
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		if attempt < maxRetries {
			backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	results <- newCheckResult(url, statusFailure, 429, "too many requests", start)
}
