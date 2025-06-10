// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// EvalLogLine matches the structure from evals/record.go
type EvalLogLine struct {
	Name      string  `json:"name"`
	Timestamp string  `json:"timestamp"`
	RunNumber int     `json:"run_number"`
	Rubric    string  `json:"rubric"`
	Output    string  `json:"output"`
	Reasoning string  `json:"reasoning"`
	Score     float64 `json:"score"`
	Pass      bool    `json:"pass"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "run":
		runCommand(args)
	case "view":
		viewCommand(args)
	default:
		// Default to view command for backward compatibility
		viewCommand(os.Args[1:])
	}
}

func printUsage() {
	fmt.Println("evalviewer - Display evaluation results from evals.jsonl")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  evalviewer run [go test flags and args]   Run tests with GOEVALS=1 then display results")
	fmt.Println("  evalviewer view [flags]                   Display existing results")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  evalviewer run -v ./conversations         Run evals for conversations package")
	fmt.Println("  evalviewer run -v ./...                   Run all evals")
	fmt.Println("  evalviewer view -file evals.jsonl         View existing results")
	fmt.Println("  evalviewer view -failures-only            Show only failures")
}

func runCommand(args []string) {
	// Execute go test with GOEVALS=1
	fmt.Println("Running evaluations...")

	// Prepare go test command
	cmdArgs := []string{"test"}
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Env = append(os.Environ(), "GOEVALS=1")

	// Run command silently, only capturing exit status
	if err := cmd.Run(); err != nil {
		fmt.Printf("Tests completed with errors: %v\n", err)
	} else {
		fmt.Println("Tests completed successfully.")
	}

	// Find and display results
	evalFile, err := findEvalsFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError finding evals.jsonl: %v\n", err)
		fmt.Println("You can view results manually with: evalviewer view -file /path/to/evals.jsonl")
		os.Exit(1)
	}

	// Display results with default settings
	results, err := loadResults(evalFile, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading results: %v\n", err)
		os.Exit(1)
	}

	// Launch TUI
	if err := runTUI(results); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func viewCommand(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	var filename string
	var maxWidth int
	var showOnlyFailures bool

	fs.StringVar(&filename, "file", "evals.jsonl", "Path to the evals.jsonl file")
	fs.IntVar(&maxWidth, "width", 80, "Maximum width for output columns")
	fs.BoolVar(&showOnlyFailures, "failures-only", false, "Show only failed evaluations")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	results, err := loadResults(filename, false) // Don't pre-filter, let TUI handle it
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading results: %v\n", err)
		os.Exit(1)
	}

	// Launch TUI
	if err := runTUI(results); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func findEvalsFile() (string, error) {
	// Look for evals.jsonl in current directory and parent directories
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		evalFile := filepath.Join(dir, "evals.jsonl")
		if _, err := os.Stat(evalFile); err == nil {
			return evalFile, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("evals.jsonl not found in current directory or parent directories")
}

func loadResults(filename string, showOnlyFailures bool) ([]EvalLogLine, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", filename, err)
	}
	defer file.Close()

	var results []EvalLogLine
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var result EvalLogLine
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing line: %v\n", err)
			continue
		}

		// Filter based on failures-only flag
		if showOnlyFailures && result.Pass {
			continue
		}

		results = append(results, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return results, nil
}
