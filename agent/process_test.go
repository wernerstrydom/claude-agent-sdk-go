package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCLINotFound(t *testing.T) {
	// Save and clear PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", oldPath)

	// Save and clear HOME to prevent checking common locations
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", "/nonexistent")
	defer os.Setenv("HOME", oldHome)

	_, err := findCLI()
	if err == nil {
		t.Error("findCLI() should return error when CLI not found")
	}

	startErr, ok := err.(*StartError)
	if !ok {
		t.Errorf("error should be *StartError, got %T", err)
	}

	if !strings.Contains(startErr.Reason, "not found") {
		t.Errorf("error reason should mention 'not found', got: %s", startErr.Reason)
	}
}

func TestFindCLIInPath(t *testing.T) {
	// Create a temporary directory with a fake claude executable
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Create executable file
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Add to PATH
	oldPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+oldPath)
	defer os.Setenv("PATH", oldPath)

	path, err := findCLI()
	if err != nil {
		t.Errorf("findCLI() error = %v, want nil", err)
	}

	if path != fakeClaude {
		t.Errorf("findCLI() = %q, want %q", path, fakeClaude)
	}
}

func TestStartProcessWithCLIPath(t *testing.T) {
	// Create a fake CLI that just exits
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Create a script that outputs JSON and exits
	script := `#!/bin/sh
echo '{"type":"system","subtype":"init","session_id":"test-123"}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}

	// Should have valid process
	if p == nil {
		t.Fatal("startProcess() returned nil process")
	}

	// Clean up
	p.close()
}

func TestStartProcessInvalidCLI(t *testing.T) {
	cfg := &config{
		model:   "test-model",
		workDir: ".",
		cliPath: "/nonexistent/claude",
	}

	ctx := context.Background()
	_, err := startProcess(ctx, cfg)
	if err == nil {
		t.Error("startProcess() should return error for invalid CLI path")
	}

	startErr, ok := err.(*StartError)
	if !ok {
		t.Errorf("error should be *StartError, got %T", err)
	}

	if startErr.Cause == nil {
		t.Error("StartError.Cause should not be nil")
	}
}

func TestProcessWriteAndClose(t *testing.T) {
	// Create a fake CLI that reads stdin and exits
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
read line
echo '{"type":"result","result":"done"}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}

	// Write to stdin
	if err := p.write([]byte("test input\n")); err != nil {
		t.Errorf("write() error = %v", err)
	}

	// Close should succeed
	if err := p.close(); err != nil {
		t.Errorf("close() error = %v", err)
	}
}

func TestProcessReader(t *testing.T) {
	// Create a fake CLI that outputs something
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo '{"type":"system","subtype":"init"}'
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	reader := p.reader()
	if reader == nil {
		t.Error("reader() should not return nil")
	}
}

func TestProcessWait(t *testing.T) {
	// Create a fake CLI that exits immediately
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
exit 0
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}

	// Wait should return without error for clean exit
	err = p.wait()
	if err != nil {
		t.Errorf("wait() error = %v, want nil for clean exit", err)
	}
}
