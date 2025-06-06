# Eval Viewer

A CLI tool to run evaluations and display results in a nice table format.

## Usage

### Run Command (Recommended)
Run tests with `GOEVALS=1` and automatically display results:

```bash
# Run evaluations for conversations package
go run main.go run -v ./conversations

# Run all evaluations
go run main.go run -v ./...

# Run with test coverage
go run main.go run -v -cover ./conversations
```

### View Command  
Display existing evaluation results:

```bash
# View all results
go run main.go view -file ../../evals.jsonl

# Show only failures (highlighted in red)
go run main.go view -failures-only

# Adjust column widths for wider terminals  
go run main.go view -width 200
```

## Commands

### `evalviewer run [go test flags and args]`
- Executes `go test` with `GOEVALS=1` environment variable
- Streams test output in real-time
- Auto-detects and displays results table
- Shows pass/fail summary

### `evalviewer view [flags]`  
- `-file`: Path to the evals.jsonl file (default: "evals.jsonl")
- `-failures-only`: Show only failed evaluations
- `-width`: Maximum width for output columns (default: 80)

## Table Columns

- **TEST**: Test name (shortened)
- **RUBRIC**: Evaluation rubric being tested
- **OUTPUT**: LLM output being evaluated  
- **RESULT**: ✓ PASS or ✗ FAIL (failures highlighted in red)
- **SCORE**: Numeric score (0.00-1.00)
- **REASONING**: Grader LLM's reasoning for the score

## Examples

```bash
# One-command evaluation workflow
go run main.go run -v ./conversations

# View only failures from previous run
go run main.go view -failures-only

# Build and use as standalone binary
go build -o evalviewer main.go
./evalviewer run -v ./...
```