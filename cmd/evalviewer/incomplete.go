// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// incompleteMarker is written next to evals.jsonl when a package did not run
// all of its evals, so CI can tell a partial run from one where evals failed.
const incompleteMarker = "evals-incomplete.txt"

// packageResult matches the line `go test` prints when a package finishes.
var packageResult = regexp.MustCompile(`^(ok|FAIL|\?)\s+(\S+)(\s+\[(build|setup) failed\])?`)

// packageWatcher scans `go test` output for packages that did not finish
// their evals: build or setup failures, panics, and timeouts (which panic).
// Package output is not interleaved, so a panic belongs to the next package
// result line.
type packageWatcher struct {
	mu         sync.Mutex
	partial    []byte
	panicked   bool
	incomplete []string
}

func (w *packageWatcher) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.partial = append(w.partial, p...)
	for {
		i := bytes.IndexByte(w.partial, '\n')
		if i < 0 {
			return len(p), nil
		}
		w.line(string(w.partial[:i]))
		w.partial = w.partial[i+1:]
	}
}

func (w *packageWatcher) line(line string) {
	if strings.HasPrefix(line, "panic: ") {
		w.panicked = true
		return
	}
	m := packageResult.FindStringSubmatch(line)
	if m == nil {
		return
	}
	if m[1] == "FAIL" && (w.panicked || m[3] != "") {
		w.incomplete = append(w.incomplete, m[2])
	}
	w.panicked = false
}

func (w *packageWatcher) packages() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.incomplete
}

// recordIncompletePackages adds an error result per package that did not
// finish, so the report shows it, and writes incompleteMarker.
func recordIncompletePackages(packages []string) error {
	if len(packages) == 0 {
		return nil
	}
	evalFile, err := findEvalsFile()
	if err != nil {
		evalFile = "evals.jsonl"
	}

	f, err := os.OpenFile(evalFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	modulePath := goModulePath()
	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "all"
	}
	enc := json.NewEncoder(f)
	for _, pkg := range packages {
		rel := strings.TrimPrefix(strings.TrimPrefix(pkg, modulePath), "/")
		if rel == "" {
			rel = "."
		}
		if err := enc.Encode(EvalLogLine{
			Name:      fmt.Sprintf("%s/[%s]_did_not_finish", rel, provider),
			Package:   rel,
			Timestamp: time.Now().Format(time.RFC3339),
			Error:     fmt.Sprintf("the %s package failed to build, panicked, or timed out before all of its evals ran; see the job log", rel),
		}); err != nil {
			return err
		}
	}

	return os.WriteFile(filepath.Join(filepath.Dir(evalFile), incompleteMarker), []byte(strings.Join(packages, "\n")+"\n"), 0600)
}

func goModulePath() string {
	out, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
