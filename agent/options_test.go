package agent

import "testing"

func TestDefaultConfig(t *testing.T) {
	c := newConfig()

	if c.model != "claude-sonnet-4-5" {
		t.Errorf("default model = %q, want %q", c.model, "claude-sonnet-4-5")
	}

	if c.workDir != "." {
		t.Errorf("default workDir = %q, want %q", c.workDir, ".")
	}
}

func TestModelOption(t *testing.T) {
	c := newConfig(Model("claude-opus-4-5"))

	if c.model != "claude-opus-4-5" {
		t.Errorf("model = %q, want %q", c.model, "claude-opus-4-5")
	}
}

func TestWorkDirOption(t *testing.T) {
	c := newConfig(WorkDir("/tmp/project"))

	if c.workDir != "/tmp/project" {
		t.Errorf("workDir = %q, want %q", c.workDir, "/tmp/project")
	}
}

func TestCLIPathOption(t *testing.T) {
	c := newConfig(CLIPath("/usr/local/bin/claude"))

	if c.cliPath != "/usr/local/bin/claude" {
		t.Errorf("cliPath = %q, want %q", c.cliPath, "/usr/local/bin/claude")
	}
}

func TestCLIPathDefaultEmpty(t *testing.T) {
	c := newConfig()

	if c.cliPath != "" {
		t.Errorf("default cliPath = %q, want empty string", c.cliPath)
	}
}

func TestOptionsCompose(t *testing.T) {
	c := newConfig(
		Model("claude-haiku-4-5"),
		WorkDir("/custom/path"),
	)

	if c.model != "claude-haiku-4-5" {
		t.Errorf("model = %q, want %q", c.model, "claude-haiku-4-5")
	}

	if c.workDir != "/custom/path" {
		t.Errorf("workDir = %q, want %q", c.workDir, "/custom/path")
	}
}

func TestOptionsAppliedInOrder(t *testing.T) {
	// Later options should override earlier ones
	c := newConfig(
		Model("first-model"),
		Model("second-model"),
	)

	if c.model != "second-model" {
		t.Errorf("model = %q, want %q (last option should win)", c.model, "second-model")
	}
}
