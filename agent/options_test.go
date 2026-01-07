package agent

import (
	"testing"
	"time"
)

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

func TestMaxTurnsOption(t *testing.T) {
	c := newConfig(MaxTurns(10))
	if c.maxTurns != 10 {
		t.Errorf("maxTurns = %d, want 10", c.maxTurns)
	}
}

func TestMaxTurnsDefaultZero(t *testing.T) {
	c := newConfig()
	if c.maxTurns != 0 {
		t.Errorf("default maxTurns = %d, want 0 (unlimited)", c.maxTurns)
	}
}

func TestResumeOption(t *testing.T) {
	c := newConfig(Resume("sess-abc-123"))
	if c.resume != "sess-abc-123" {
		t.Errorf("resume = %q, want %q", c.resume, "sess-abc-123")
	}
	if c.fork {
		t.Error("fork should be false when using Resume")
	}
}

func TestForkOption(t *testing.T) {
	c := newConfig(Fork("sess-abc-123"))
	if c.resume != "sess-abc-123" {
		t.Errorf("resume = %q, want %q", c.resume, "sess-abc-123")
	}
	if !c.fork {
		t.Error("fork should be true when using Fork")
	}
}

func TestTimeoutRunOption(t *testing.T) {
	rc := newRunConfig(Timeout(5 * time.Second))
	if rc.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", rc.timeout)
	}
}

func TestMaxTurnsRunOption(t *testing.T) {
	rc := newRunConfig(MaxTurnsRun(5))
	if rc.maxTurns != 5 {
		t.Errorf("maxTurns = %d, want 5", rc.maxTurns)
	}
}

func TestRunConfigDefaults(t *testing.T) {
	rc := newRunConfig()
	if rc.timeout != 0 {
		t.Errorf("default timeout = %v, want 0", rc.timeout)
	}
	if rc.maxTurns != 0 {
		t.Errorf("default maxTurns = %d, want 0", rc.maxTurns)
	}
}

func TestRunOptionsCompose(t *testing.T) {
	rc := newRunConfig(
		Timeout(10*time.Second),
		MaxTurnsRun(3),
	)
	if rc.timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", rc.timeout)
	}
	if rc.maxTurns != 3 {
		t.Errorf("maxTurns = %d, want 3", rc.maxTurns)
	}
}

func TestPostToolUseOption(t *testing.T) {
	hook1 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		return HookResult{Decision: Continue}
	}
	hook2 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		return HookResult{Decision: Continue}
	}

	c := newConfig(PostToolUse(hook1, hook2))

	if len(c.postToolUseHooks) != 2 {
		t.Errorf("postToolUseHooks length = %d, want 2", len(c.postToolUseHooks))
	}
}

func TestPostToolUseOptionAccumulates(t *testing.T) {
	hook1 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		return HookResult{Decision: Continue}
	}
	hook2 := func(tc *ToolCall, tr *ToolResultContext) HookResult {
		return HookResult{Decision: Continue}
	}

	c := newConfig(
		PostToolUse(hook1),
		PostToolUse(hook2),
	)

	if len(c.postToolUseHooks) != 2 {
		t.Errorf("postToolUseHooks length = %d, want 2 (should accumulate)", len(c.postToolUseHooks))
	}
}

func TestPostToolUseDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.postToolUseHooks) != 0 {
		t.Errorf("default postToolUseHooks length = %d, want 0", len(c.postToolUseHooks))
	}
}

func TestOnStopOption(t *testing.T) {
	hook1 := func(e *StopEvent) {}
	hook2 := func(e *StopEvent) {}

	c := newConfig(OnStop(hook1, hook2))

	if len(c.stopHooks) != 2 {
		t.Errorf("stopHooks length = %d, want 2", len(c.stopHooks))
	}
}

func TestOnStopOptionAccumulates(t *testing.T) {
	hook1 := func(e *StopEvent) {}
	hook2 := func(e *StopEvent) {}

	c := newConfig(
		OnStop(hook1),
		OnStop(hook2),
	)

	if len(c.stopHooks) != 2 {
		t.Errorf("stopHooks length = %d, want 2 (should accumulate)", len(c.stopHooks))
	}
}

func TestOnStopDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.stopHooks) != 0 {
		t.Errorf("default stopHooks length = %d, want 0", len(c.stopHooks))
	}
}

func TestPreCompactOption(t *testing.T) {
	hook1 := func(e *PreCompactEvent) PreCompactResult {
		return PreCompactResult{Archive: true}
	}
	hook2 := func(e *PreCompactEvent) PreCompactResult {
		return PreCompactResult{Extract: "data"}
	}

	c := newConfig(PreCompact(hook1, hook2))

	if len(c.preCompactHooks) != 2 {
		t.Errorf("preCompactHooks length = %d, want 2", len(c.preCompactHooks))
	}
}

func TestPreCompactOptionAccumulates(t *testing.T) {
	hook1 := func(e *PreCompactEvent) PreCompactResult {
		return PreCompactResult{}
	}
	hook2 := func(e *PreCompactEvent) PreCompactResult {
		return PreCompactResult{}
	}

	c := newConfig(
		PreCompact(hook1),
		PreCompact(hook2),
	)

	if len(c.preCompactHooks) != 2 {
		t.Errorf("preCompactHooks length = %d, want 2 (should accumulate)", len(c.preCompactHooks))
	}
}

func TestPreCompactDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.preCompactHooks) != 0 {
		t.Errorf("default preCompactHooks length = %d, want 0", len(c.preCompactHooks))
	}
}

func TestSubagentStopOption(t *testing.T) {
	hook1 := func(e *SubagentStopEvent) {}
	hook2 := func(e *SubagentStopEvent) {}

	c := newConfig(SubagentStop(hook1, hook2))

	if len(c.subagentStopHooks) != 2 {
		t.Errorf("subagentStopHooks length = %d, want 2", len(c.subagentStopHooks))
	}
}

func TestSubagentStopOptionAccumulates(t *testing.T) {
	hook1 := func(e *SubagentStopEvent) {}
	hook2 := func(e *SubagentStopEvent) {}

	c := newConfig(
		SubagentStop(hook1),
		SubagentStop(hook2),
	)

	if len(c.subagentStopHooks) != 2 {
		t.Errorf("subagentStopHooks length = %d, want 2 (should accumulate)", len(c.subagentStopHooks))
	}
}

func TestSubagentStopDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.subagentStopHooks) != 0 {
		t.Errorf("default subagentStopHooks length = %d, want 0", len(c.subagentStopHooks))
	}
}

func TestUserPromptSubmitOption(t *testing.T) {
	hook1 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{UpdatedPrompt: e.Prompt + "!"}
	}
	hook2 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{Metadata: "test"}
	}

	c := newConfig(UserPromptSubmit(hook1, hook2))

	if len(c.userPromptSubmitHooks) != 2 {
		t.Errorf("userPromptSubmitHooks length = %d, want 2", len(c.userPromptSubmitHooks))
	}
}

func TestUserPromptSubmitOptionAccumulates(t *testing.T) {
	hook1 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{}
	}
	hook2 := func(e *PromptSubmitEvent) PromptSubmitResult {
		return PromptSubmitResult{}
	}

	c := newConfig(
		UserPromptSubmit(hook1),
		UserPromptSubmit(hook2),
	)

	if len(c.userPromptSubmitHooks) != 2 {
		t.Errorf("userPromptSubmitHooks length = %d, want 2 (should accumulate)", len(c.userPromptSubmitHooks))
	}
}

func TestUserPromptSubmitDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.userPromptSubmitHooks) != 0 {
		t.Errorf("default userPromptSubmitHooks length = %d, want 0", len(c.userPromptSubmitHooks))
	}
}

func TestSubagentOption(t *testing.T) {
	c := newConfig(Subagent("tester",
		SubagentDescription("Runs tests"),
		SubagentTools("Bash", "Read"),
		SubagentModel("haiku"),
	))

	if len(c.subagents) != 1 {
		t.Fatalf("subagents length = %d, want 1", len(c.subagents))
	}
	sub, ok := c.subagents["tester"]
	if !ok {
		t.Fatal("subagent 'tester' not found")
	}
	if sub.Name != "tester" {
		t.Errorf("subagent name = %q, want %q", sub.Name, "tester")
	}
	if sub.Description != "Runs tests" {
		t.Errorf("subagent description = %q, want %q", sub.Description, "Runs tests")
	}
	if len(sub.Tools) != 2 || sub.Tools[0] != "Bash" || sub.Tools[1] != "Read" {
		t.Errorf("subagent tools = %v, want [Bash Read]", sub.Tools)
	}
	if sub.Model != "haiku" {
		t.Errorf("subagent model = %q, want %q", sub.Model, "haiku")
	}
}

func TestSubagentOptionMultiple(t *testing.T) {
	c := newConfig(
		Subagent("explorer", SubagentDescription("Explores codebase")),
		Subagent("writer", SubagentDescription("Writes code")),
	)

	if len(c.subagents) != 2 {
		t.Fatalf("subagents length = %d, want 2", len(c.subagents))
	}
	if _, ok := c.subagents["explorer"]; !ok {
		t.Error("subagent 'explorer' not found")
	}
	if _, ok := c.subagents["writer"]; !ok {
		t.Error("subagent 'writer' not found")
	}
}

func TestSubagentDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.subagents) != 0 {
		t.Errorf("default subagents should be empty, got %v", c.subagents)
	}
}

func TestSkillOption(t *testing.T) {
	content := "# Go Skill\nUse gofmt."
	c := newConfig(Skill("go", content))

	if len(c.skills) != 1 {
		t.Fatalf("skills length = %d, want 1", len(c.skills))
	}
	skill, ok := c.skills["go"]
	if !ok {
		t.Fatal("skill 'go' not found")
	}
	if skill.Name != "go" {
		t.Errorf("skill name = %q, want %q", skill.Name, "go")
	}
	if skill.Content != content {
		t.Errorf("skill content = %q, want %q", skill.Content, content)
	}
}

func TestSkillOptionMultiple(t *testing.T) {
	c := newConfig(
		Skill("go", "Go content"),
		Skill("python", "Python content"),
	)

	if len(c.skills) != 2 {
		t.Fatalf("skills length = %d, want 2", len(c.skills))
	}
	if _, ok := c.skills["go"]; !ok {
		t.Error("skill 'go' not found")
	}
	if _, ok := c.skills["python"]; !ok {
		t.Error("skill 'python' not found")
	}
}

func TestSkillDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.skills) != 0 {
		t.Errorf("default skills should be empty, got %v", c.skills)
	}
}

func TestSkillsDirOption(t *testing.T) {
	c := newConfig(SkillsDir("./skills"))

	if len(c.skillDirs) != 1 {
		t.Fatalf("skillDirs length = %d, want 1", len(c.skillDirs))
	}
	if c.skillDirs[0] != "./skills" {
		t.Errorf("skillDirs[0] = %q, want %q", c.skillDirs[0], "./skills")
	}
}

func TestSkillsDirOptionAccumulates(t *testing.T) {
	c := newConfig(
		SkillsDir("./skills1"),
		SkillsDir("./skills2"),
	)

	if len(c.skillDirs) != 2 {
		t.Fatalf("skillDirs length = %d, want 2", len(c.skillDirs))
	}
}

func TestSkillsDirDefaultEmpty(t *testing.T) {
	c := newConfig()

	if len(c.skillDirs) != 0 {
		t.Errorf("default skillDirs length = %d, want 0", len(c.skillDirs))
	}
}

func TestSystemPromptPresetOption(t *testing.T) {
	c := newConfig(SystemPromptPreset("concise"))

	if c.systemPromptPreset != "concise" {
		t.Errorf("systemPromptPreset = %q, want %q", c.systemPromptPreset, "concise")
	}
}

func TestSystemPromptPresetDefaultEmpty(t *testing.T) {
	c := newConfig()

	if c.systemPromptPreset != "" {
		t.Errorf("default systemPromptPreset = %q, want empty", c.systemPromptPreset)
	}
}

func TestSystemPromptAppendOption(t *testing.T) {
	c := newConfig(SystemPromptAppend("Always explain your reasoning."))

	if c.systemPromptAppend != "Always explain your reasoning." {
		t.Errorf("systemPromptAppend = %q, want %q", c.systemPromptAppend, "Always explain your reasoning.")
	}
}

func TestSystemPromptAppendDefaultEmpty(t *testing.T) {
	c := newConfig()

	if c.systemPromptAppend != "" {
		t.Errorf("default systemPromptAppend = %q, want empty", c.systemPromptAppend)
	}
}
