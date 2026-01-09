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

func TestToolsOption(t *testing.T) {
	c := newConfig(Tools("Bash", "Read", "Write"))

	if len(c.tools) != 3 {
		t.Fatalf("tools length = %d, want 3", len(c.tools))
	}
	if c.tools[0] != "Bash" || c.tools[1] != "Read" || c.tools[2] != "Write" {
		t.Errorf("tools = %v, want [Bash Read Write]", c.tools)
	}
}

func TestAllowedToolsOption(t *testing.T) {
	c := newConfig(AllowedTools("Bash(git:*)", "Read"))

	if len(c.allowedTools) != 2 {
		t.Fatalf("allowedTools length = %d, want 2", len(c.allowedTools))
	}
	if c.allowedTools[0] != "Bash(git:*)" {
		t.Errorf("allowedTools[0] = %q, want %q", c.allowedTools[0], "Bash(git:*)")
	}
}

func TestDisallowedToolsOption(t *testing.T) {
	c := newConfig(DisallowedTools("Bash(rm:*)", "Write"))

	if len(c.disallowedTools) != 2 {
		t.Fatalf("disallowedTools length = %d, want 2", len(c.disallowedTools))
	}
	if c.disallowedTools[0] != "Bash(rm:*)" {
		t.Errorf("disallowedTools[0] = %q, want %q", c.disallowedTools[0], "Bash(rm:*)")
	}
}

func TestPermissionPromptOption(t *testing.T) {
	tests := []struct {
		mode PermissionMode
		want PermissionMode
	}{
		{PermissionDefault, PermissionDefault},
		{PermissionAcceptEdits, PermissionAcceptEdits},
		{PermissionBypass, PermissionBypass},
		{PermissionDontAsk, PermissionDontAsk},
		{PermissionPlan, PermissionPlan},
	}

	for _, tt := range tests {
		c := newConfig(PermissionPrompt(tt.mode))
		if c.permissionMode != tt.want {
			t.Errorf("PermissionPrompt(%v): got %v, want %v", tt.mode, c.permissionMode, tt.want)
		}
	}
}

func TestPermissionModeDefault(t *testing.T) {
	c := newConfig()

	if c.permissionMode != PermissionDefault {
		t.Errorf("default permissionMode = %q, want %q", c.permissionMode, PermissionDefault)
	}
}

func TestEnvOption(t *testing.T) {
	c := newConfig(
		Env("TMPDIR", "/sandbox/tmp"),
		Env("HOME", "/sandbox/home"),
	)

	if len(c.env) != 2 {
		t.Fatalf("env length = %d, want 2", len(c.env))
	}
	if c.env["TMPDIR"] != "/sandbox/tmp" {
		t.Errorf("env[TMPDIR] = %q, want %q", c.env["TMPDIR"], "/sandbox/tmp")
	}
	if c.env["HOME"] != "/sandbox/home" {
		t.Errorf("env[HOME] = %q, want %q", c.env["HOME"], "/sandbox/home")
	}
}

func TestEnvOptionAccumulates(t *testing.T) {
	// Multiple Env calls should accumulate
	c := newConfig(
		Env("VAR1", "value1"),
		Env("VAR2", "value2"),
		Env("VAR1", "updated"), // Override previous
	)

	if len(c.env) != 2 {
		t.Fatalf("env length = %d, want 2", len(c.env))
	}
	if c.env["VAR1"] != "updated" {
		t.Errorf("env[VAR1] = %q, want %q (should be overridden)", c.env["VAR1"], "updated")
	}
}

func TestEnvDefaultEmpty(t *testing.T) {
	c := newConfig()

	if c.env == nil {
		t.Error("default env should be initialized, got nil")
	}
	if len(c.env) != 0 {
		t.Errorf("default env length = %d, want 0", len(c.env))
	}
}

func TestAddDirOption(t *testing.T) {
	c := newConfig(AddDir("/data", "/shared"))

	if len(c.addDirs) != 2 {
		t.Fatalf("addDirs length = %d, want 2", len(c.addDirs))
	}
	if c.addDirs[0] != "/data" || c.addDirs[1] != "/shared" {
		t.Errorf("addDirs = %v, want [/data /shared]", c.addDirs)
	}
}

func TestAddDirOptionAccumulates(t *testing.T) {
	c := newConfig(
		AddDir("/first"),
		AddDir("/second", "/third"),
	)

	if len(c.addDirs) != 3 {
		t.Fatalf("addDirs length = %d, want 3", len(c.addDirs))
	}
}

func TestSettingSourcesOption(t *testing.T) {
	c := newConfig(SettingSources("user", "project"))

	if len(c.settingSources) != 2 {
		t.Fatalf("settingSources length = %d, want 2", len(c.settingSources))
	}
	if c.settingSources[0] != "user" || c.settingSources[1] != "project" {
		t.Errorf("settingSources = %v, want [user project]", c.settingSources)
	}
}
