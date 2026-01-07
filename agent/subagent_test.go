package agent

import "testing"

func TestSubagentConfig(t *testing.T) {
	cfg := &SubagentConfig{Name: "test"}

	SubagentDescription("A test subagent")(cfg)
	if cfg.Description != "A test subagent" {
		t.Errorf("expected description 'A test subagent', got %q", cfg.Description)
	}

	SubagentPrompt("You are a tester")(cfg)
	if cfg.Prompt != "You are a tester" {
		t.Errorf("expected prompt 'You are a tester', got %q", cfg.Prompt)
	}

	SubagentTools("Bash", "Read")(cfg)
	if len(cfg.Tools) != 2 || cfg.Tools[0] != "Bash" || cfg.Tools[1] != "Read" {
		t.Errorf("expected tools [Bash, Read], got %v", cfg.Tools)
	}

	SubagentModel("haiku")(cfg)
	if cfg.Model != "haiku" {
		t.Errorf("expected model 'haiku', got %q", cfg.Model)
	}
}

func TestSubagentOptionsCompose(t *testing.T) {
	cfg := &SubagentConfig{Name: "composed"}

	opts := []SubagentOption{
		SubagentDescription("Runs tests"),
		SubagentTools("Bash"),
		SubagentModel("haiku"),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Description != "Runs tests" {
		t.Errorf("expected description 'Runs tests', got %q", cfg.Description)
	}
	if len(cfg.Tools) != 1 || cfg.Tools[0] != "Bash" {
		t.Errorf("expected tools [Bash], got %v", cfg.Tools)
	}
	if cfg.Model != "haiku" {
		t.Errorf("expected model 'haiku', got %q", cfg.Model)
	}
}
