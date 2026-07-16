package skillsconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const projectConfigRelativePath = ".cursor/skills.yaml"

// Config 描述全局或项目级 skill 配置。
type Config struct {
	Global  []Entry `yaml:"global,omitempty"`
	Project []Entry `yaml:"project,omitempty"`
}

// Entry 描述单个 skill 来源。
type Entry struct {
	Name    string `yaml:"name,omitempty"`
	Path    string `yaml:"path,omitempty"`
	GitHub  string `yaml:"github,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty"`
}

// ResolvedSkill 是已物化到本地的 skill。
type ResolvedSkill struct {
	Name        string
	Description string
	FolderPath  string
	ReadmePath  string
	Scope       Scope
	Source      string
}

// Scope 区分全局与项目 skill。
type Scope string

const (
	ScopeGlobal  Scope = "global"
	ScopeProject Scope = "project"
)

func (entry Entry) enabled() bool {
	if entry.Enabled == nil {
		return true
	}
	return *entry.Enabled
}

func loadConfigFile(path string) (Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Config{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read skills config %q: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Config{}, nil
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse skills config %q: %w", path, err)
	}
	return cfg, nil
}

func mergeConfigs(configs ...Config) Config {
	merged := Config{}
	for _, cfg := range configs {
		merged.Global = append(merged.Global, cfg.Global...)
		merged.Project = append(merged.Project, cfg.Project...)
	}
	return merged
}

func discoverGlobalConfigPaths() []string {
	paths := make([]string, 0, 2)
	if fakehome := strings.TrimSpace(os.Getenv("CURSOR_BYOK_FAKEHOME")); fakehome != "" {
		repoSkills := filepath.Join(filepath.Dir(fakehome), "skills.yaml")
		paths = append(paths, repoSkills)
	}
	paths = append(paths, defaultGlobalConfigPath())
	return dedupePaths(paths)
}

func defaultGlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ""
	}
	return filepath.Join(home, ".cursor-local-assistant-v2", "skills.yaml")
}

func projectConfigPath(workspaceRoot string) string {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return ""
	}
	return filepath.Join(workspaceRoot, projectConfigRelativePath)
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func loadMergedConfig(workspaceRoot string) (Config, map[string]string, error) {
	sources := make(map[string]string)
	merged := Config{}

	for _, path := range discoverGlobalConfigPaths() {
		cfg, err := loadConfigFile(path)
		if err != nil {
			return Config{}, nil, err
		}
		if len(cfg.Global) > 0 || len(cfg.Project) > 0 {
			merged.Global = append(merged.Global, cfg.Global...)
			merged.Project = append(merged.Project, cfg.Project...)
			sources["global:"+path] = path
		}
	}

	projectPath := projectConfigPath(workspaceRoot)
	if projectPath != "" {
		cfg, err := loadConfigFile(projectPath)
		if err != nil {
			return Config{}, nil, err
		}
		if len(cfg.Project) > 0 {
			merged.Project = append(merged.Project, cfg.Project...)
			sources["project:"+projectPath] = projectPath
		}
		if len(cfg.Global) > 0 {
			merged.Global = append(merged.Global, cfg.Global...)
			sources["global:"+projectPath] = projectPath
		}
	}
	return merged, sources, nil
}

func entriesForScope(cfg Config, scope Scope) []Entry {
	switch scope {
	case ScopeProject:
		return cfg.Project
	default:
		return cfg.Global
	}
}
