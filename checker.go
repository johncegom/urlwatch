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
	Index      int         `json:"index"`
	Url        string      `json:"url"`
	Status     checkStatus `json:"status"`
	StatusCode int         `json:"statusCode"` // 0 if the request never got a response at all
	ErrMsg     string      `json:"errMsg"`     // empty if statusCode is set and valid
	Latency    int64       `json:"latencyMs"`
	CheckedAt  time.Time   `json:"checkedAt"`
}

func newCheckResult(index int, url string, status checkStatus, statusCode int, errMsg string, start time.Time) checkResult {
	return checkResult{
		Index:      index,
		Url:        url,
		Status:     status,
		StatusCode: statusCode,
		ErrMsg:     errMsg,
		Latency:    time.Since(start).Milliseconds(),
		CheckedAt:  time.Now(),
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

func checkURL(job job, results chan<- checkResult, cfg checkerConfig) {
	start := time.Now()

	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := doOneAttempt(job.url, cfg)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				results <- newCheckResult(job.index, job.url, statusFailure, 0, "timeout", start)
				return
			}
			results <- newCheckResult(job.index, job.url, statusFailure, 0, err.Error(), start)
			return
		}

		status := classify(resp.StatusCode)
		errMsg := ""

		if status == statusReachable {
			errMsg = "may require authentication"
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			results <- newCheckResult(job.index, job.url, status, resp.StatusCode, errMsg, start)
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		if attempt < maxRetries {
			backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	results <- newCheckResult(job.index, job.url, statusFailure, 429, "too many requests", start)
}
