package process

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
)

// OutputLine represents a line of output from a command
type OutputLine struct {
	Stream  string // "stdout" or "stderr"
	Content string
}

// Runner executes shell commands and manages their output
type Runner struct {
	verbose bool
}

// NewRunner creates a new Runner instance
func NewRunner() *Runner {
	return &Runner{}
}

// SetVerbose enables/disables verbose output
func (r *Runner) SetVerbose(v bool) {
	r.verbose = v
}

// Run executes a command and streams output line by line
// Returns channels for output lines and a final error
func (r *Runner) Run(ctx context.Context, name string, args []string) (<-chan OutputLine, <-chan error) {
	outChan := make(chan OutputLine, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		defer close(errChan)

		cmd := exec.CommandContext(ctx, name, args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			errChan <- fmt.Errorf("stdout pipe: %w", err)
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			errChan <- fmt.Errorf("stderr pipe: %w", err)
			return
		}

		if err := cmd.Start(); err != nil {
			errChan <- fmt.Errorf("start: %w", err)
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Stream stdout
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				case outChan <- OutputLine{Stream: "stdout", Content: scanner.Text()}:
				}
			}
		}()

		// Stream stderr
		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				case outChan <- OutputLine{Stream: "stderr", Content: scanner.Text()}:
				}
			}
		}()

		wg.Wait()

		if err := cmd.Wait(); err != nil {
			errChan <- err
			return
		}
	}()

	return outChan, errChan
}

// RunSilent executes a command and returns the combined output
func (r *Runner) RunSilent(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Include stderr in error for debugging
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%w: %s", err, stderr.String())
		}
		return nil, err
	}

	return stdout.Bytes(), nil
}

// RunJSON executes a command and parses the JSON output into v
func (r *Runner) RunJSON(ctx context.Context, name string, args []string, v any) error {
	output, err := r.RunSilent(ctx, name, args)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(output, v); err != nil {
		return fmt.Errorf("json parse: %w", err)
	}

	return nil
}

// CommandExists checks if a command is available on PATH
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
