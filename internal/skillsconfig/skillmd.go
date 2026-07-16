package skillsconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func resolveSkillDirectory(sourcePath string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("skill source path is empty")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		readme := filepath.Join(sourcePath, "SKILL.md")
		if _, err := os.Stat(readme); err != nil {
			return "", fmt.Errorf("skill directory missing SKILL.md: %s", sourcePath)
		}
		return sourcePath, nil
	}
	if strings.EqualFold(filepath.Base(sourcePath), "SKILL.md") {
		return filepath.Dir(sourcePath), nil
	}
	return "", fmt.Errorf("skill path must be a directory or SKILL.md: %s", sourcePath)
}

func readSkillMetadata(skillDir string) (skillFrontmatter, error) {
	readmePath := filepath.Join(skillDir, "SKILL.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return skillFrontmatter{}, err
	}
	meta, body := parseFrontmatter(string(data))
	if strings.TrimSpace(meta.Name) == "" {
		meta.Name = filepath.Base(skillDir)
	}
	if strings.TrimSpace(meta.Description) == "" {
		meta.Description = firstMeaningfulLine(body)
	}
	if strings.TrimSpace(meta.Description) == "" {
		return skillFrontmatter{}, fmt.Errorf("skill %q has empty description", skillDir)
	}
	return meta, nil
}

func parseFrontmatter(content string) (skillFrontmatter, string) {
	content = strings.TrimPrefix(content, "\uFEFF")
	if !strings.HasPrefix(content, "---") {
		return skillFrontmatter{}, content
	}
	rest := content[len("---"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return skillFrontmatter{}, content
	}
	front := strings.TrimSpace(rest[:end])
	body := strings.TrimPrefix(rest[end+len("\n---"):], "\n")
	var meta skillFrontmatter
	_ = yaml.Unmarshal([]byte(front), &meta)
	return meta, body
}

func firstMeaningfulLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line
	}
	return ""
}
