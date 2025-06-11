# Eval Viewer

A CLI tool to run evaluations and display results in a TUI (Terminal User Interface).

## Installation

### From Local Repository (Recommended)
```bash
cd cmd/evalviewer
go install
```

After installation, the `evalviewer` command will be available in your PATH.

## Usage

### Run Command (Recommended)
Run go test with `GOEVALS=1` environment variable set, then automatically find and display the evaluation results in a TUI.

All arguments after 'run' are passed directly to 'go test'.

```bash
# Run evaluations for conversations package
evalviewer run ./conversations

# Run all evaluations
evalviewer run -v ./...

# Run with test coverage
evalviewer run -cover ./conversations
```

The run command will:
1. Execute go test with GOEVALS=1
2. Search for evals.jsonl in current and parent directories
3. Launch the TUI to display results

### View Command  
Display evaluation results from an existing evals.jsonl file in a TUI.

```bash
# View existing results (defaults to evals.jsonl in current directory)
evalviewer view

# View results from specific file
evalviewer view -file evals.jsonl
evalviewer view -f /path/to/evals.jsonl

# Show only failed evaluations
evalviewer view -failures-only
```
