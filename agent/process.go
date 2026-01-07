package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

	// Tool configuration
	if len(cfg.tools) > 0 {
		args = append(args, "--tools", strings.Join(cfg.tools, ","))
	}
	if len(cfg.allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(cfg.allowedTools, ","))
	}
	if len(cfg.disallowedTools) > 0 {
		args = append(args, "--disallowedTools", strings.Join(cfg.disallowedTools, ","))
	}

	// Permission mode
	if cfg.permissionMode != "" && cfg.permissionMode != PermissionDefault {
		args = append(args, "--permission-mode", string(cfg.permissionMode))
	}

	// Additional directories
	if len(cfg.addDirs) > 0 {
		for _, dir := range cfg.addDirs {
			args = append(args, "--add-dir", dir)
		}
	}

	// Setting sources
	if len(cfg.settingSources) > 0 {
		args = append(args, "--setting-sources", strings.Join(cfg.settingSources, ","))
	}

	// Session management
	if cfg.resume != "" {
		args = append(args, "--resume", cfg.resume)
		if cfg.fork {
			args = append(args, "--fork-session")
		}
	}

	// Structured output
	if cfg.jsonSchema != "" {
		args = append(args, "--json-schema", cfg.jsonSchema)
	}

	// MCP server configuration
	for name, mcp := range cfg.mcpServers {
		var mcpJSON map[string]any
		switch mcp.Transport {
		case "stdio", "":
			mcpJSON = map[string]any{
				"command": mcp.Command,
				"args":    mcp.Args,
			}
			if len(mcp.Env) > 0 {
				mcpJSON["env"] = mcp.Env
			}
		case "sse", "http":
			mcpJSON = map[string]any{
				"type": mcp.Transport,
				"url":  mcp.URL,
			}
			if len(mcp.Headers) > 0 {
				mcpJSON["headers"] = mcp.Headers
			}
		}
		jsonBytes, _ := json.Marshal(map[string]any{name: mcpJSON})
		args = append(args, "--mcp-config", string(jsonBytes))
	}

	// Strict MCP config
	if cfg.strictMCPConfig {
		args = append(args, "--strict-mcp-config")
	}

	// System prompt configuration
	if cfg.systemPromptPreset != "" {
		args = append(args, "--system-prompt-preset", cfg.systemPromptPreset)
	}
	if cfg.systemPromptAppend != "" {
		args = append(args, "--append-system-prompt", cfg.systemPromptAppend)
	}

	// Load skills from directories
	allSkills := make(map[string]*SkillConfig)
	for name, skill := range cfg.skills {
		allSkills[name] = skill
	}
	for _, dir := range cfg.skillDirs {
		dirSkills, err := loadSkillsFromDir(dir)
		if err != nil {
			return nil, &StartError{Reason: "failed to load skills from " + dir, Cause: err}
		}
		for name, skill := range dirSkills {
			allSkills[name] = skill
		}
	}
	// Note: Skills are typically loaded into context via system prompt or config
	// The CLI may support --skill flag or require skills in a config file
	// For now, we append skill content to system prompt if present
	if len(allSkills) > 0 && cfg.systemPromptAppend == "" {
		var skillContent strings.Builder
		for name, skill := range allSkills {
			skillContent.WriteString("\n\n## Skill: ")
			skillContent.WriteString(name)
			skillContent.WriteString("\n")
			skillContent.WriteString(skill.Content)
		}
		args = append(args, "--append-system-prompt", skillContent.String())
	} else if len(allSkills) > 0 {
		// Skills were requested but systemPromptAppend is already set
		// Combine them
		var skillContent strings.Builder
		skillContent.WriteString(cfg.systemPromptAppend)
		for name, skill := range allSkills {
			skillContent.WriteString("\n\n## Skill: ")
			skillContent.WriteString(name)
			skillContent.WriteString("\n")
			skillContent.WriteString(skill.Content)
		}
		// Find and replace the existing --append-system-prompt arg
		for i, arg := range args {
			if arg == "--append-system-prompt" && i+1 < len(args) {
				args[i+1] = skillContent.String()
				break
			}
		}
	}

	// Subagent configuration
	// Note: Subagents are typically defined via Task tool configuration
	// The exact CLI flag depends on Claude Code's implementation
	for name, sub := range cfg.subagents {
		subJSON := map[string]any{
			"name": name,
		}
		if sub.Description != "" {
			subJSON["description"] = sub.Description
		}
		if sub.Prompt != "" {
			subJSON["prompt"] = sub.Prompt
		}
		if len(sub.Tools) > 0 {
			subJSON["tools"] = sub.Tools
		}
		if sub.Model != "" {
			subJSON["model"] = sub.Model
		}
		jsonBytes, _ := json.Marshal(map[string]any{name: subJSON})
		args = append(args, "--subagent", string(jsonBytes))
	}

	cmd := exec.CommandContext(ctx, cliPath, args...) // #nosec G204 -- CLI path is validated in New()
	cmd.Dir = cfg.workDir

	// Create a new process group so we can kill all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Environment variables - start with current environment, then add/override
	if len(cfg.env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range cfg.env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	// Create pipes
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, &StartError{Reason: "failed to create stdin pipe", Cause: err}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close() // Best-effort cleanup
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
		_ = stdin.Close()  // Best-effort cleanup
		_ = stdout.Close() // Best-effort cleanup
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
		_ = p.stdin.Close() // Best-effort; signal EOF to child process
	}

	killed := false

	// Wait with timeout
	select {
	case <-p.done:
		// Process exited
	case <-time.After(5 * time.Second):
		// Kill the entire process group to ensure child processes are also killed.
		// This is necessary because shell scripts may spawn child processes (like sleep)
		// that would continue running and keep pipes open if we only killed the parent.
		if p.cmd.Process != nil {
			// Kill process group using negative PID
			_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL) // Best-effort termination
			killed = true
		}
		// Close stdout to unblock any IO goroutines reading from it.
		// This is necessary because cmd.Wait() won't return until all
		// pipe goroutines complete. On Unix systems, killing a process
		// doesn't automatically close the read end of pipes.
		if p.stdout != nil {
			_ = p.stdout.Close() // Best-effort; unblock IO goroutines
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
