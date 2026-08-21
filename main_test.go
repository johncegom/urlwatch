package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/johncegom/urlwatch/checker"
)

func TestProcessResults_Text_AllHealthy(t *testing.T) {
	var buf bytes.Buffer
	results := []checker.CheckResult{
		{Index: 0, Url: "https://a.com", Status: checker.StatusHealthy, StatusCode: 200},
	}

	hasFailure := processResults(&buf, results, false)

	if hasFailure {
		t.Errorf("Got hasFailure = true, want false")
	}

	if !strings.Contains(buf.String(), "https://a.com is healthy(200)") {
		t.Errorf("got %v, want %v", buf.String(), "https://a.com is healthy(200)")
	}
}

func TestProcessResults_JSON_AllFailure(t *testing.T) {
	var buf bytes.Buffer
	results := []checker.CheckResult{
		{Index: 0, Url: "https://test.com", Status: checker.StatusFailure, StatusCode: 500},
		{Index: 1, Url: "https://test1.com", Status: checker.StatusFailure, StatusCode: 501},
		{Index: 2, Url: "https://test2.com", Status: checker.StatusFailure, StatusCode: 505},
	}

	hasFailure := processResults(&buf, results, true)

	if !hasFailure {
		t.Errorf("got hasFailure = false, want true")
	}

	var got []checker.CheckResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output was not valid json: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d results, want 3", len(got))
	}

	for _, g := range got {
		if g.Status != checker.StatusFailure {
			t.Errorf("got status %v, want %v", g.Status, checker.StatusFailure)
		}
	}
}

func TestProcessResults_JSON_SortedResult(t *testing.T) {
	var buf bytes.Buffer
	results := []checker.CheckResult{
		{Index: 1, Url: "https://test1.com", Status: checker.StatusHealthy, StatusCode: 200},
		{Index: 0, Url: "https://test.com", Status: checker.StatusReachable, StatusCode: 401},
		{Index: 3, Url: "https://test3.com", Status: checker.StatusHealthy, StatusCode: 200},
		{Index: 2, Url: "https://test2.com", Status: checker.StatusHealthy, StatusCode: 200},
	}

	hasFailure := processResults(&buf, results, true)

	if hasFailure {
		t.Errorf("Got hasFailure = true, want false")
	}

	var got []checker.CheckResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output was not valid json: %v", err)
	}

	for i, g := range got {
		if g.Index != i {
			t.Errorf("got %v, want %v", g.Index, i)
		}
	}
}
