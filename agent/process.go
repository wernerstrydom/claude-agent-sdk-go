package agent

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// process manages the Claude Code CLI subprocess.
type process struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  bytes.Buffer
	done    chan struct{}
	exitErr error
	mu      sync.Mutex
}

// findCLI locates the Claude CLI executable.
func findCLI() (string, error) {
	// Check PATH first
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	commonPaths := []string{
		filepath.Join(home, ".npm-global", "bin", "claude"),
		"/usr/local/bin/claude",
		filepath.Join(home, ".local", "bin", "claude"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", &StartError{Reason: "claude CLI not found in PATH or common locations"}
}

// startProcess spawns the Claude Code CLI process.
func startProcess(ctx context.Context, cfg *config) (*process, error) {
	// Find CLI path
	cliPath := cfg.cliPath
	if cliPath == "" {
		var err error
		cliPath, err = findCLI()
		if err != nil {
			return nil, err
		}
	}

	// Build command arguments
	args := []string{
		"--print", "-",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
	}
	if cfg.model != "" {
		args = append(args, "--model", cfg.model)
	}

	cmd := exec.CommandContext(ctx, cliPath, args...)
	cmd.Dir = cfg.workDir

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, &StartError{Reason: "failed to create stdin pipe", Cause: err}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, &StartError{Reason: "failed to create stdout pipe", Cause: err}
	}

	p := &process{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		done:   make(chan struct{}),
	}

	// Capture stderr
	cmd.Stderr = &p.stderr

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, &StartError{Reason: "failed to start claude CLI", Cause: err}
	}

	// Launch goroutine to wait for exit
	go func() {
		p.exitErr = cmd.Wait()
		close(p.done)
	}()

	return p, nil
}

// write sends data to the process stdin.
func (p *process) write(data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, err := p.stdin.Write(data)
	return err
}

// reader returns the stdout reader.
func (p *process) reader() io.Reader {
	return p.stdout
}

// close terminates the process gracefully.
func (p *process) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close stdin to signal EOF
	if p.stdin != nil {
		p.stdin.Close()
	}

	killed := false

	// Wait with timeout
	select {
	case <-p.done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Kill if still running
		if p.cmd.Process != nil {
			p.cmd.Process.Kill()
			killed = true
		}
		<-p.done
	}

	// Check exit status - ignore if we killed the process
	if p.exitErr != nil && !killed {
		if exitErr, ok := p.exitErr.(*exec.ExitError); ok {
			// Only report error for non-signal exits when not killed
			if exitErr.ExitCode() > 0 {
				return &ProcessError{
					ExitCode: exitErr.ExitCode(),
					Stderr:   p.stderr.String(),
				}
			}
		}
	}

	return nil
}

// wait blocks until the process exits.
func (p *process) wait() error {
	<-p.done
	return p.exitErr
}
