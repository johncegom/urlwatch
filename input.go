package main

import (
	"bufio"
	"flag"
	"fmt"
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

func readURLs(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("file open failed: %w", err)
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file stopped: %w", err)
	}

	return urls, nil
}
