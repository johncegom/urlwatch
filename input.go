package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
)

func parseFlags() string {
	filePath := flag.String("file", "", "path to the file contains all urls need to be checked")
	flag.Parse()

	if *filePath == "" {
		flag.Usage()
		os.Exit(1)
	}
	return *filePath
}

func readURLs(filePath string) ([]string, []string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("file open failed: %w", err)
	}
	defer file.Close()

	var urls []string
	var warnings []string

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		u := scanner.Text()
		if _, err := url.ParseRequestURI(u); err != nil {
			warnings = append(warnings, fmt.Sprintf("line %d: invalid url %q", lineNumber, u))
			continue
		}
		urls = append(urls, u)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading file stopped: %w", err)
	}

	return urls, warnings, nil
}
