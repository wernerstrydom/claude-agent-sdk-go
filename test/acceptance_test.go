//go:build acceptance

// Package test contains acceptance tests that verify the SDK works with the real Claude CLI.
// Run with: go test -tags=acceptance -v ./test/...
//
// Prerequisites:
// - Claude CLI installed and authenticated
// - Go, Python, GCC/Clang, Java (for respective language tests)
//
// These tests create real files and make real API calls. They are not run by default.
package test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wernerstrydom/claude-agent-sdk-go/agent"
)

// testTimeout is the maximum time for each test
const testTimeout = 2 * time.Minute

// TestHelloWorldGo verifies the agent can create and we can run a Go program.
func TestHelloWorldGo(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World program in Go.
Just create main.go that prints "Hello, World!" to stdout.
Do not create any other files. Do not use modules.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	mainGo := filepath.Join(workDir, "main.go")
	if _, err := os.Stat(mainGo); os.IsNotExist(err) {
		t.Fatal("main.go was not created")
	}

	// Verify it compiles and runs
	out, err := exec.CommandContext(ctx, "go", "run", mainGo).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run Go program: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("Go program output: %s", strings.TrimSpace(string(out)))
}

// TestHelloWorldPython verifies the agent can create and we can run a Python program.
func TestHelloWorldPython(t *testing.T) {
	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
		if _, err := exec.LookPath(pythonCmd); err != nil {
			t.Skip("python not installed")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World program in Python.
Just create hello.py that prints "Hello, World!" to stdout.
Do not create any other files.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	helloPy := filepath.Join(workDir, "hello.py")
	if _, err := os.Stat(helloPy); os.IsNotExist(err) {
		t.Fatal("hello.py was not created")
	}

	// Verify it runs
	out, err := exec.CommandContext(ctx, pythonCmd, helloPy).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run Python program: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("Python program output: %s", strings.TrimSpace(string(out)))
}

// TestHelloWorldC verifies the agent can create and we can compile/run a C program.
func TestHelloWorldC(t *testing.T) {
	compiler := "gcc"
	if _, err := exec.LookPath(compiler); err != nil {
		compiler = "clang"
		if _, err := exec.LookPath(compiler); err != nil {
			t.Skip("gcc/clang not installed")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World program in C.
Just create hello.c that prints "Hello, World!" to stdout.
Do not create any other files.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	helloC := filepath.Join(workDir, "hello.c")
	if _, err := os.Stat(helloC); os.IsNotExist(err) {
		t.Fatal("hello.c was not created")
	}

	// Compile
	binaryPath := filepath.Join(workDir, "hello")
	compileOut, err := exec.CommandContext(ctx, compiler, "-o", binaryPath, helloC).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile C program: %v\nOutput: %s", err, compileOut)
	}

	// Run
	out, err := exec.CommandContext(ctx, binaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run C program: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("C program output: %s", strings.TrimSpace(string(out)))
}

// TestHelloWorldJava verifies the agent can create and we can compile/run a Java program.
func TestHelloWorldJava(t *testing.T) {
	if _, err := exec.LookPath("javac"); err != nil {
		t.Skip("javac not installed")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("java not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World program in Java.
Create HelloWorld.java that prints "Hello, World!" to stdout.
Use a public class named HelloWorld with a main method.
Do not create any other files.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	helloJava := filepath.Join(workDir, "HelloWorld.java")
	if _, err := os.Stat(helloJava); os.IsNotExist(err) {
		t.Fatal("HelloWorld.java was not created")
	}

	// Compile
	compileOut, err := exec.CommandContext(ctx, "javac", helloJava).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile Java program: %v\nOutput: %s", err, compileOut)
	}

	// Run
	out, err := exec.CommandContext(ctx, "java", "-cp", workDir, "HelloWorld").CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run Java program: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("Java program output: %s", strings.TrimSpace(string(out)))
}

// TestHelloWorldRust verifies the agent can create and we can compile/run a Rust program.
func TestHelloWorldRust(t *testing.T) {
	if _, err := exec.LookPath("rustc"); err != nil {
		t.Skip("rustc not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World program in Rust.
Just create hello.rs that prints "Hello, World!" to stdout.
Do not use Cargo. Do not create any other files.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	helloRs := filepath.Join(workDir, "hello.rs")
	if _, err := os.Stat(helloRs); os.IsNotExist(err) {
		t.Fatal("hello.rs was not created")
	}

	// Compile
	binaryPath := filepath.Join(workDir, "hello")
	compileOut, err := exec.CommandContext(ctx, "rustc", "-o", binaryPath, helloRs).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to compile Rust program: %v\nOutput: %s", err, compileOut)
	}

	// Run
	out, err := exec.CommandContext(ctx, binaryPath).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run Rust program: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("Rust program output: %s", strings.TrimSpace(string(out)))
}

// TestPreToolUseDenial verifies that a PreToolUse hook can deny tool execution.
// When the agent tries to use a denied tool, it should either self-correct
// or the run completes with a result indicating the denial.
func TestPreToolUseDenial(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	var deniedCount int
	denyWrite := func(tc *agent.ToolCall) agent.HookResult {
		if tc.Name == "Write" {
			deniedCount++
			return agent.HookResult{
				Decision: agent.Deny,
				Reason:   "Writing files is not allowed; use Bash echo instead",
			}
		}
		return agent.HookResult{Decision: agent.Continue}
	}

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			denyWrite,
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	// Ask the agent to write a file - the Write tool should be denied.
	// The agent should self-correct and use Bash instead.
	result, err := a.Run(ctx, `Create a file called hello.txt containing "Hello, World!".
If the Write tool is denied, use Bash with echo/printf to create the file instead.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if deniedCount == 0 {
		t.Log("Agent did not attempt to use Write tool (used Bash directly)")
	} else {
		t.Logf("Write tool was denied %d time(s); agent self-corrected", deniedCount)
	}

	// Verify the file was created (by whatever means)
	helloTxt := filepath.Join(workDir, "hello.txt")
	content, err := os.ReadFile(helloTxt)
	if err != nil {
		t.Fatalf("hello.txt was not created: %v", err)
	}

	if !strings.Contains(string(content), "Hello") {
		t.Errorf("hello.txt content does not contain 'Hello': %s", content)
	}

	t.Logf("Result: %s (cost: $%.4f)", result.ResultText, result.CostUSD)
}

// TestMultiTurnConversation verifies session continuity across multiple
// Stream() and Run() calls on the same agent.
func TestMultiTurnConversation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	// Turn 1: Stream a request to set a variable via a file
	var firstResult *agent.Result
	for msg := range a.Stream(ctx, `Create a file called context.txt containing exactly "secret_value_42". Do not add any newline at the end.`) {
		if r, ok := msg.(*agent.Result); ok {
			firstResult = r
		}
	}
	if err := a.Err(); err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	if firstResult == nil {
		t.Fatal("No result from Stream")
	}

	t.Logf("Turn 1 result: %s (cost: $%.4f)", firstResult.ResultText, firstResult.CostUSD)

	// Turn 2: Use Run() to ask about the file from the previous turn
	secondResult, err := a.Run(ctx, `Read the file context.txt and tell me its exact contents. Reply with just the contents, nothing else.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !strings.Contains(secondResult.ResultText, "secret_value_42") {
		t.Errorf("Second turn did not recall first turn context; got: %s", secondResult.ResultText)
	}

	t.Logf("Turn 2 result: %s (cost: $%.4f)", secondResult.ResultText, secondResult.CostUSD)
}

// TestStructuredOutput verifies RunStructured returns properly typed data.
func TestStructuredOutput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	type MathAnswer struct {
		Value       int    `json:"value" desc:"The numeric result"`
		Explanation string `json:"explanation" desc:"Brief explanation of the calculation"`
	}

	var answer MathAnswer
	result, err := agent.RunStructured(ctx, "What is 17 + 25? Respond with the numeric value and a brief explanation.", &answer,
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
	)
	if err != nil {
		t.Fatalf("RunStructured failed: %v", err)
	}

	if answer.Value != 42 {
		t.Errorf("Value = %d, want 42", answer.Value)
	}

	if answer.Explanation == "" {
		t.Error("Explanation is empty")
	}

	t.Logf("Structured result: value=%d, explanation=%q (cost: $%.4f)",
		answer.Value, answer.Explanation, result.CostUSD)
}

// TestHelloWorldBash verifies the agent can create and we can run a Bash script.
func TestHelloWorldBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	workDir := t.TempDir()

	a, err := agent.New(ctx,
		agent.WorkDir(workDir),
		agent.PermissionPrompt(agent.PermissionAcceptEdits),
		agent.PreToolUse(
			agent.DenyCommands("sudo", "rm -rf", "curl", "wget", "ssh", "scp", "nc", "netcat"),
		),
	)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer a.Close()

	_, err = a.Run(ctx, `Create a Hello World Bash script.
Create hello.sh that prints "Hello, World!" to stdout.
Make it executable with a proper shebang.
Do not create any other files.`)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify file exists
	helloSh := filepath.Join(workDir, "hello.sh")
	if _, err := os.Stat(helloSh); os.IsNotExist(err) {
		t.Fatal("hello.sh was not created")
	}

	// Run
	out, err := exec.CommandContext(ctx, "bash", helloSh).CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run Bash script: %v\nOutput: %s", err, out)
	}

	if !strings.Contains(strings.ToLower(string(out)), "hello") {
		t.Errorf("Output does not contain 'hello': %s", out)
	}

	t.Logf("Bash script output: %s", strings.TrimSpace(string(out)))
}
