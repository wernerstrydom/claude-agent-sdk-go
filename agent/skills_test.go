package agent

import (
	"path/filepath"
	"testing"
)

func TestLoadSkillsFromDir(t *testing.T) {
	// Create temp directory structure
	dir := t.TempDir()

	// Create skill using legacy SKILL.md format
	legacyDir := filepath.Join(dir, "go-dev")
	mustMkdirAll(t, legacyDir, 0o755)
	mustWriteFile(t, filepath.Join(legacyDir, "SKILL.md"), []byte("# Go Development\nUse gofmt."), 0o644)

	// Create skill using .skill.md format
	mustWriteFile(t, filepath.Join(dir, "testing.skill.md"), []byte("# Testing\nWrite tests first."), 0o644)

	// Create non-skill file (should be ignored)
	mustWriteFile(t, filepath.Join(dir, "readme.md"), []byte("# README"), 0o644)

	skills, err := loadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("loadSkillsFromDir failed: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}

	// Check legacy format
	if skill, ok := skills["go-dev"]; !ok {
		t.Error("expected skill 'go-dev' from legacy SKILL.md")
	} else if skill.Content != "# Go Development\nUse gofmt." {
		t.Errorf("unexpected content for go-dev skill: %q", skill.Content)
	}

	// Check new format
	if skill, ok := skills["testing"]; !ok {
		t.Error("expected skill 'testing' from .skill.md")
	} else if skill.Content != "# Testing\nWrite tests first." {
		t.Errorf("unexpected content for testing skill: %q", skill.Content)
	}
}

func TestLoadSkillsFromDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	skills, err := loadSkillsFromDir(dir)
	if err != nil {
		t.Fatalf("loadSkillsFromDir failed: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected 0 skills, got %d", len(skills))
	}
}

func TestLoadSkillsFromDir_NonExistent(t *testing.T) {
	_, err := loadSkillsFromDir("/nonexistent/path/12345")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}
