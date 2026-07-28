package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		statusCode int
		want       checkStatus
	}{
		{199, statusFailure},
		{200, statusHealthy},
		{299, statusHealthy},
		{300, statusFailure},
		{301, statusFailure},
		{401, statusReachable},
		{403, statusReachable},
		{404, statusFailure},
		{500, statusFailure},
	}

	for _, c := range cases {
		got := classify(c.statusCode)
		if got != c.want {
			t.Errorf("classify(%d) = %v, want %v \n", c.statusCode, got, c.want)
		}
	}
}

func TestCheckURL_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	results := make(chan checkResult, 1)
	checkURL(server.URL, results, 10*time.Millisecond)
	result := <-results

	if result.status != statusHealthy {
		t.Errorf("got status %v, want %v", result.status, statusHealthy)
	}
	if result.statusCode != 200 {
		t.Errorf("got status code = %v, want status code = 200", result.statusCode)
	}
	if result.errMsg != "" {
		t.Errorf("got error message %v, want empty", result.errMsg)
	}
}

func TestCheckURL_Reachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	results := make(chan checkResult, 1)
	checkURL(server.URL, results, 10*time.Millisecond)

	result := <-results

	if result.status != statusReachable || result.statusCode != 403 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusReachable, 403)
	}

	if result.errMsg == "" {
		t.Errorf("got empty errMsg, want a real error message")
	}
}

func TestCheckURL_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		time.Sleep(30 * time.Millisecond)
	}))
	defer server.Close()

	results := make(chan checkResult, 1)
	checkURL(server.URL, results, 10*time.Millisecond)

	result := <-results

	if result.status != statusFailure {
		t.Errorf("got status %v, want %v", result.status, statusFailure)
	}
	if result.statusCode != 0 {
		t.Errorf("got status code = %v, want status code = 0", result.statusCode)
	}
	if result.errMsg != "timeout" {
		t.Errorf("got error message %v, want timeout", result.errMsg)
	}
}

func TestCheckURL_RetriesOn429_ThenSucceeds(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	results := make(chan checkResult, 1)
	checkURL(server.URL, results, 200*time.Millisecond)
	result := <-results

	if result.status != statusHealthy || result.statusCode != 200 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusHealthy, 200)
	}
	if callCount != 3 {
		t.Errorf("got call %v times, want 3 times", callCount)
	}
}

func TestCheckURL_RetriesOn429_ThenExhausts(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	results := make(chan checkResult, 1)
	checkURL(server.URL, results, 200*time.Millisecond)
	result := <-results

	if result.status != statusFailure || result.statusCode != 429 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusFailure, 429)
	}

	if callCount != 4 {
		t.Errorf("got call %v times, want 4 times", callCount)
	}
}

func TestCheckURL_ConnectionRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close() // close immediately so nothing is listening when checkURL dials it

	results := make(chan checkResult, 1)
	checkURL(url, results, 200*time.Millisecond)
	result := <-results

	if result.status != statusFailure || result.statusCode != 0 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusFailure, 0)
	}
	if result.errMsg == "" {
		t.Errorf("got empty errMsg, want a real connection error message")
	}
}

func TestCheckURL_EmptyURL(t *testing.T) {
	results := make(chan checkResult, 1)
	checkURL("", results, 200*time.Millisecond)
	result := <-results

	if result.status != statusFailure || result.statusCode != 0 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusFailure, 0)
	}
	if result.errMsg == "" {
		t.Errorf("got empty errMsg, want a real error message")
	}
}

func TestCheckURL_MalformedURL_ControlCharacter(t *testing.T) {
	badURL := "http://example.com/\r\nX-Injected: true"

	results := make(chan checkResult, 1)
	checkURL(badURL, results, 200*time.Millisecond)
	result := <-results

	if result.status != statusFailure || result.statusCode != 0 {
		t.Errorf("got status: %v and statusCode = %v, want status: %v and statusCode = %v", result.status, result.statusCode, statusFailure, 0)
	}
	if result.errMsg == "" {
		t.Errorf("got empty errMsg, want a real error message")
	}
}

func FuzzCheckURL(f *testing.F) {
	f.Add("https://example.com")
	f.Add("")
	f.Add("not-a-url")
	f.Add("http://example.com/\r\nX-Injected: true")

	f.Fuzz(func(t *testing.T, url string) {
		results := make(chan checkResult, 1)
		checkURL(url, results, 50*time.Millisecond)

		select {
		case result := <-results:
			if result.url != url {
				t.Errorf("got result.url = %q, want %q", result.url, url)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("checkURL never sent a result - possible hang")
		}
	})
}
