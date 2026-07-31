package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
)

const maxFileSize = 10 * 1024 * 1024 // 10MB
const maxURLs = 10000
const maxLineSize = 1 * 1024 * 1024 // 1MB per line
const maxWorkers int = 100

type cliConfig struct {
	filePath   string
	numWorkers int
}

func parseFlags() cliConfig {
	filePath := flag.String("file", "", "path to the file contains all urls need to be checked")
	numWorkers := flag.Int("workers", 5, "number of concurrency workers")
	flag.Parse()

	if *filePath == "" {
		flag.Usage()
		os.Exit(1)
	}

	if *numWorkers <= 0 {
		fmt.Fprintf(os.Stderr, "error: -workers %d must be positive number\n", *numWorkers)
		os.Exit(1)
	}

	if *numWorkers > maxWorkers {
		fmt.Fprintf(os.Stderr, "error: -workers %d exceeds the maximum of %d\n", *numWorkers, maxWorkers)
		os.Exit(1)
	}

	return cliConfig{
		filePath:   *filePath,
		numWorkers: *numWorkers,
	}
}

func readURLs(filePath string) ([]string, []string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("file check failed: %w", err)
	}
	if info.Size() > maxFileSize {
		return nil, nil, fmt.Errorf("file check failed: file exceeds file size limit (%d)", maxFileSize)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("file open failed: %w", err)
	}
	defer file.Close()

	var urls []string
	var warnings []string

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineSize)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		if len(urls) >= maxURLs {
			return nil, nil, fmt.Errorf("file check failed: number of urls exceeds limit (%d)", maxURLs)
		}

		u, err := url.ParseRequestURI(line)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid url %q", lineNumber, line))
			continue
		}

		if u.Scheme != "https" && u.Scheme != "http" {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid url (non-valid scheme) %q", lineNumber, line))
			continue
		}

		urls = append(urls, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading file stopped: %w", err)
	}

	return urls, warnings, nil
}
