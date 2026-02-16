package skills

import (
	"testing"
)

func TestCodingSkillParsing(t *testing.T) {
	loader := NewLoader([]string{"../../skills/builtin"}, "")

	skills, err := loader.ListSkills()
	if err != nil {
		t.Fatalf("ListSkills failed: %v", err)
	}

	// Find coding skill
	var codingSkill *Skill
	for _, s := range skills {
		if s.Name == "coding" {
			codingSkill = s
			break
		}
	}

	if codingSkill == nil {
		t.Fatal("coding skill not found")
	}

	t.Logf("Coding skill found:")
	t.Logf("  Name: %s", codingSkill.Name)
	t.Logf("  Description: %s", codingSkill.Description)
	t.Logf("  Always: %v", codingSkill.Always)
	t.Logf("  Emoji: %s", codingSkill.Emoji)
	t.Logf("  Available: %v", codingSkill.Available)

	if !codingSkill.Always {
		t.Error("coding skill should have Always=true")
	}

	// Test loading full content
	fullSkill, err := loader.LoadSkill("coding")
	if err != nil {
		t.Fatalf("LoadSkill failed: %v", err)
	}

	t.Logf("  Content length: %d bytes", len(fullSkill.Content))
	t.Logf("  Content preview: %s...", truncate(fullSkill.Content, 100))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
