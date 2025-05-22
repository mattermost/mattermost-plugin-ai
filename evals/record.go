// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package evals

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type EvalLogLine struct {
	Name      string  `json:"name"`
	Timestamp string  `json:"timestamp"`
	RunNumber int     `json:"run_number"`
	Reasoning string  `json:"reasoning"`
	Score     float64 `json:"score"`
	Pass      bool    `json:"pass"`
}

type EvalResult struct {
	Reasoning string  `json:"reasoning"`
	Score     float64 `json:"score"`
	Pass      bool    `json:"pass"`
}

// evalFileLock is a mutex to prevent concurrent writes to the eval results file.
var evalFileLock sync.Mutex

// RecordScore records the score of an eval in a JSONL file.
func RecordScore(e *EvalT, result *EvalResult) {
	e.Helper()

	log := EvalLogLine{
		Name:      e.Name(),
		Timestamp: time.Now().Format(time.RFC3339),
		RunNumber: e.runNumber,
		Reasoning: result.Reasoning,
		Score:     result.Score,
		Pass:      result.Pass,
	}

	e.Logf("Eval result: %+v", log)

	dir, err := findCurrentModuleRoot()
	if err != nil {
		e.Fatalf("Failed to find module root: %v", err)
		return
	}
	file := filepath.Join(dir, "evals.jsonl")

	evalFileLock.Lock()
	defer evalFileLock.Unlock()

	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		e.Fatalf("Failed to open evals.jsonl: %v", err)
		return
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(log); err != nil {
		e.Fatalf("Failed to write to evals.jsonl: %v", err)
		return
	}
}

func findCurrentModuleRoot() (string, error) {
	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Find the module root
	moduleRoot := findModuleRoot(dir)
	if moduleRoot == "" {
		return "", errors.New("module root not found")
	}

	return moduleRoot, nil
}

// findProjectRoot finds the root of the project by looking for a go.mod file.
// Taken from go tool source cmd/go/internal/modload/init.go
func findModuleRoot(dir string) (roots string) {
	if dir == "" {
		panic("dir not set")
	}
	dir = filepath.Clean(dir)

	// Look for enclosing go.mod.
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		d := filepath.Dir(dir)
		if d == dir {
			break
		}
		dir = d
	}
	return ""
}
