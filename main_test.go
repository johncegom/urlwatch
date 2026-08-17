package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestProcessResults_Text_AllHealthy(t *testing.T) {
	var buf bytes.Buffer
	results := []checkResult{
		{Index: 0, Url: "https://a.com", Status: statusHealthy, StatusCode: 200},
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
	results := []checkResult{
		{Index: 0, Url: "https://test.com", Status: statusFailure, StatusCode: 500},
		{Index: 1, Url: "https://test1.com", Status: statusFailure, StatusCode: 501},
		{Index: 2, Url: "https://test2.com", Status: statusFailure, StatusCode: 505},
	}

	hasFailure := processResults(&buf, results, true)

	if !hasFailure {
		t.Errorf("got hasFailure = false, want true")
	}

	var got []checkResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output was not valid json: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d results, want 3", len(got))
	}

	for _, g := range got {
		if g.Status != statusFailure {
			t.Errorf("got status %v, want %v", g.Status, statusFailure)
		}
	}
}

func TestProcessResults_JSON_SortedResult(t *testing.T) {
	var buf bytes.Buffer
	results := []checkResult{
		{Index: 1, Url: "https://test1.com", Status: statusHealthy, StatusCode: 200},
		{Index: 0, Url: "https://test.com", Status: statusReachable, StatusCode: 401},
		{Index: 3, Url: "https://test3.com", Status: statusHealthy, StatusCode: 200},
		{Index: 2, Url: "https://test2.com", Status: statusHealthy, StatusCode: 200},
	}

	hasFailure := processResults(&buf, results, true)

	if hasFailure {
		t.Errorf("Got hasFailure = true, want false")
	}

	var got []checkResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output was not valid json: %v", err)
	}

	for i, g := range got {
		if g.Index != i {
			t.Errorf("got %v, want %v", g.Index, i)
		}
	}
}
