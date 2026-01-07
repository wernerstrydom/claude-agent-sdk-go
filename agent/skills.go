package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// SkillConfig holds a skill definition.
// Skills are markdown instructions that are loaded into Claude's context.
type SkillConfig struct {
	Name    string // Skill name (key for skill lookup)
	Content string // Markdown content of the skill
}

// loadSkillsFromDir walks a directory and loads all skill files.
// Skill files can be named SKILL.md (legacy) or *.skill.md.
// The skill name is derived from the filename or parent directory.
func loadSkillsFromDir(path string) (map[string]*SkillConfig, error) {
	skills := make(map[string]*SkillConfig)

	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		var skillName string

		// Check for SKILL.md in directory (legacy format)
		if strings.EqualFold(name, "SKILL.md") {
			// Use parent directory name as skill name
			skillName = filepath.Base(filepath.Dir(p))
		} else if strings.HasSuffix(name, ".skill.md") {
			// Use filename without extension as skill name
			skillName = strings.TrimSuffix(name, ".skill.md")
		} else {
			// Not a skill file
			return nil
		}

		content, err := os.ReadFile(p) // #nosec G304 -- Path from filepath.Walk within user-provided directory
		if err != nil {
			return err
		}

		skills[skillName] = &SkillConfig{
			Name:    skillName,
			Content: string(content),
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return skills, nil
}
