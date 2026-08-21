package checker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"code.dny.dev/ssrf"
)

type CheckStatus string

const (
	StatusHealthy   CheckStatus = "healthy"
	StatusReachable CheckStatus = "reachable"
	StatusFailure   CheckStatus = "failure"
)

type CheckResult struct {
	Index      int         `json:"index"`
	Url        string      `json:"url"`
	Status     CheckStatus `json:"status"`
	StatusCode int         `json:"statusCode"` // 0 if the request never got a response at all
	ErrMsg     string      `json:"errMsg"`     // empty if statusCode is set and valid
	Latency    int64       `json:"latencyMs"`
	CheckedAt  time.Time   `json:"checkedAt"`
}

func newCheckResult(index int, url string, status CheckStatus, statusCode int, errMsg string, start time.Time) CheckResult {
	return CheckResult{
		Index:      index,
		Url:        url,
		Status:     status,
		StatusCode: statusCode,
		ErrMsg:     errMsg,
		Latency:    time.Since(start).Milliseconds(),
		CheckedAt:  time.Now(),
	}
}

type CheckerConfig struct {
	Timeout time.Duration
	Client  *http.Client
}

func NewProductionConfig(timeout time.Duration) CheckerConfig {
	return CheckerConfig{
		Timeout: timeout,
		Client:  SafeClient,
	}
}

var SafeClient = newSafeClient()

func newSafeClient() *http.Client {
	guardian := ssrf.New()
	dialer := &net.Dialer{Control: guardian.Safe}
	transport := &http.Transport{DialContext: dialer.DialContext}
	return &http.Client{Transport: transport}
}

func classify(statusCode int) CheckStatus {
	if statusCode >= 200 && statusCode < 300 {
		return StatusHealthy
	} else if statusCode == 401 || statusCode == 403 {
		return StatusReachable
	} else {
		return StatusFailure
	}
}

func doOneAttempt(url string, cfg CheckerConfig) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	return cfg.Client.Do(req)
}

func CheckURL(url string, index int, cfg CheckerConfig) CheckResult {
	start := time.Now()

	const maxRetries = 3

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := doOneAttempt(url, cfg)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return newCheckResult(index, url, StatusFailure, 0, "timeout", start)

			}
			return newCheckResult(index, url, StatusFailure, 0, err.Error(), start)
		}

		status := classify(resp.StatusCode)
		errMsg := ""

		if status == StatusReachable {
			errMsg = "may require authentication"
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			resp.Body.Close()
			return newCheckResult(index, url, status, resp.StatusCode, errMsg, start)
		}
		resp.Body.Close()

		if attempt < maxRetries {
			backoff := time.Duration(100*(1<<attempt)) * time.Millisecond
			time.Sleep(backoff)
		}
	}

	return newCheckResult(index, url, StatusFailure, 429, "too many requests", start)
}
