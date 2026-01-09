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

// TestStartProcess_ToolsFlag verifies --tools flag is added to CLI args.
func TestStartProcess_ToolsFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Script that outputs its arguments
	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
		tools:   []string{"Bash", "Read", "Write"},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	// Wait for script to write args
	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--tools") {
		t.Errorf("args should contain --tools, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "Bash,Read,Write") {
		t.Errorf("args should contain tool list, got: %s", argsStr)
	}
}

// TestStartProcess_AllowedToolsFlag verifies --allowedTools flag is added.
func TestStartProcess_AllowedToolsFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:        "test-model",
		workDir:      tmpDir,
		cliPath:      fakeClaude,
		allowedTools: []string{"Bash(git:*)", "Read"},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--allowedTools") {
		t.Errorf("args should contain --allowedTools, got: %s", argsStr)
	}
}

// TestStartProcess_DisallowedToolsFlag verifies --disallowedTools flag is added.
func TestStartProcess_DisallowedToolsFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:           "test-model",
		workDir:         tmpDir,
		cliPath:         fakeClaude,
		disallowedTools: []string{"Bash(rm:*)"},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--disallowedTools") {
		t.Errorf("args should contain --disallowedTools, got: %s", argsStr)
	}
}

// TestStartProcess_PermissionModeFlag verifies --permission-mode flag is added.
func TestStartProcess_PermissionModeFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:          "test-model",
		workDir:        tmpDir,
		cliPath:        fakeClaude,
		permissionMode: PermissionAcceptEdits,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--permission-mode") {
		t.Errorf("args should contain --permission-mode, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "acceptEdits") {
		t.Errorf("args should contain acceptEdits, got: %s", argsStr)
	}
}

// TestStartProcess_PermissionModeDefault verifies default mode doesn't add flag.
func TestStartProcess_PermissionModeDefault(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:          "test-model",
		workDir:        tmpDir,
		cliPath:        fakeClaude,
		permissionMode: PermissionDefault,
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if strings.Contains(argsStr, "--permission-mode") {
		t.Errorf("args should NOT contain --permission-mode for default, got: %s", argsStr)
	}
}

// TestStartProcess_AddDirFlag verifies --add-dir flags are added.
func TestStartProcess_AddDirFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
		addDirs: []string{"/data", "/shared"},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--add-dir") {
		t.Errorf("args should contain --add-dir, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "/data") {
		t.Errorf("args should contain /data, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "/shared") {
		t.Errorf("args should contain /shared, got: %s", argsStr)
	}
}

// TestStartProcess_SettingSourcesFlag verifies --setting-sources flag is added.
func TestStartProcess_SettingSourcesFlag(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	script := `#!/bin/sh
echo "$@" > ` + filepath.Join(tmpDir, "args.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:          "test-model",
		workDir:        tmpDir,
		cliPath:        fakeClaude,
		settingSources: []string{"user", "project"},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	args, err := os.ReadFile(filepath.Join(tmpDir, "args.txt"))
	if err != nil {
		t.Fatalf("failed to read args: %v", err)
	}

	argsStr := string(args)
	if !strings.Contains(argsStr, "--setting-sources") {
		t.Errorf("args should contain --setting-sources, got: %s", argsStr)
	}
	if !strings.Contains(argsStr, "user,project") {
		t.Errorf("args should contain user,project, got: %s", argsStr)
	}
}

// TestStartProcess_EnvMerge verifies environment variables are merged.
func TestStartProcess_EnvMerge(t *testing.T) {
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Script that outputs specific env vars
	script := `#!/bin/sh
echo "CUSTOM_VAR=$CUSTOM_VAR" > ` + filepath.Join(tmpDir, "env.txt") + `
echo "ANOTHER_VAR=$ANOTHER_VAR" >> ` + filepath.Join(tmpDir, "env.txt") + `
sleep 0.1
`
	if err := os.WriteFile(fakeClaude, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := &config{
		model:   "test-model",
		workDir: tmpDir,
		cliPath: fakeClaude,
		env: map[string]string{
			"CUSTOM_VAR":  "custom_value",
			"ANOTHER_VAR": "another_value",
		},
	}

	ctx := context.Background()
	p, err := startProcess(ctx, cfg)
	if err != nil {
		t.Fatalf("startProcess() error = %v", err)
	}
	defer p.close()

	p.wait()

	env, err := os.ReadFile(filepath.Join(tmpDir, "env.txt"))
	if err != nil {
		t.Fatalf("failed to read env: %v", err)
	}

	envStr := string(env)
	if !strings.Contains(envStr, "CUSTOM_VAR=custom_value") {
		t.Errorf("env should contain CUSTOM_VAR=custom_value, got: %s", envStr)
	}
	if !strings.Contains(envStr, "ANOTHER_VAR=another_value") {
		t.Errorf("env should contain ANOTHER_VAR=another_value, got: %s", envStr)
	}
}
