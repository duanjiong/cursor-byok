package skillsconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cursor/internal/appdata"
	"cursor/internal/logger"
)

// Resolver 根据 skills.yaml 解析 skill，直接返回源路径（不 symlink / 不 copy）。
type Resolver struct {
	cacheRoot string
	mu        sync.Mutex
}

func NewResolver() *Resolver {
	return &Resolver{
		cacheRoot: appdata.SkillsCacheRootPath(),
	}
}

func (resolver *Resolver) Resolve(workspaceRoot string) ([]ResolvedSkill, error) {
	if resolver == nil {
		return nil, nil
	}
	cfg, sources, err := loadMergedConfig(workspaceRoot)
	if err != nil {
		return nil, err
	}
	if len(cfg.Global) == 0 && len(cfg.Project) == 0 {
		return nil, nil
	}

	resolver.mu.Lock()
	defer resolver.mu.Unlock()

	byName := make(map[string]ResolvedSkill)
	ordered := make([]string, 0, len(cfg.Global)+len(cfg.Project))

	resolveScope := func(entries []Entry, scope Scope, configBase string) {
		for _, entry := range entries {
			if !entry.enabled() {
				continue
			}
			skill, err := resolver.resolveEntry(entry, scope, configBase)
			if err != nil {
				logger.Infof("skip configured skill scope=%s source=%q err=%v", scope, entrySourceLabel(entry), err)
				continue
			}
			if existing, ok := byName[skill.Name]; ok {
				logger.Infof("override configured skill name=%q old=%s new=%s", skill.Name, existing.Source, skill.Source)
			} else {
				ordered = append(ordered, skill.Name)
			}
			byName[skill.Name] = skill
		}
	}

	globalBase := firstConfigDir(sources, "global:")
	projectBase := firstConfigDir(sources, "project:")
	if projectBase == "" {
		projectBase = workspaceRoot
	}
	resolveScope(cfg.Global, ScopeGlobal, globalBase)
	if strings.TrimSpace(workspaceRoot) != "" {
		resolveScope(cfg.Project, ScopeProject, projectBase)
	}

	result := make([]ResolvedSkill, 0, len(ordered))
	for _, name := range ordered {
		if skill, ok := byName[name]; ok {
			result = append(result, skill)
		}
	}
	return result, nil
}

func (resolver *Resolver) resolveEntry(entry Entry, scope Scope, configBase string) (ResolvedSkill, error) {
	sourcePath, sourceLabel, err := resolver.materializeSource(entry, configBase)
	if err != nil {
		return ResolvedSkill{}, err
	}
	skillDir, err := resolveSkillDirectory(sourcePath)
	if err != nil {
		return ResolvedSkill{}, err
	}
	meta, err := readSkillMetadata(skillDir)
	if err != nil {
		return ResolvedSkill{}, err
	}
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = strings.TrimSpace(meta.Name)
	}
	if name == "" {
		name = filepath.Base(skillDir)
	}
	if err := validateSkillName(name); err != nil {
		return ResolvedSkill{}, err
	}

	readmePath := filepath.Join(skillDir, "SKILL.md")
	return ResolvedSkill{
		Name:        name,
		Description: strings.TrimSpace(meta.Description),
		FolderPath:  skillDir,
		ReadmePath:  readmePath,
		Scope:       scope,
		Source:      sourceLabel,
	}, nil
}

func (resolver *Resolver) materializeSource(entry Entry, configBase string) (string, string, error) {
	githubSource := strings.TrimSpace(entry.GitHub)
	localPath := strings.TrimSpace(entry.Path)
	switch {
	case githubSource != "" && localPath != "":
		return "", "", fmt.Errorf("skill entry must specify either github or path")
	case githubSource != "":
		cacheDir, err := syncGitHubSource(resolver.cacheRoot, githubSource)
		if err != nil {
			return "", "", err
		}
		return cacheDir, "github:" + githubSource, nil
	case localPath != "":
		resolved := localPath
		if !filepath.IsAbs(resolved) {
			base := strings.TrimSpace(configBase)
			if base == "" {
				base, _ = os.Getwd()
			}
			resolved = filepath.Join(base, localPath)
		}
		resolved = filepath.Clean(resolved)
		if _, err := os.Stat(resolved); err != nil {
			return "", "", fmt.Errorf("local skill path unavailable: %w", err)
		}
		return resolved, "path:" + resolved, nil
	default:
		return "", "", fmt.Errorf("skill entry requires github or path")
	}
}

func validateSkillName(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return fmt.Errorf("skill name is required")
	case strings.Contains(name, "/"), strings.Contains(name, "\\"), strings.Contains(name, ".."):
		return fmt.Errorf("invalid skill name %q", name)
	default:
		return nil
	}
}

func entrySourceLabel(entry Entry) string {
	if strings.TrimSpace(entry.GitHub) != "" {
		return entry.GitHub
	}
	return entry.Path
}

func firstConfigDir(sources map[string]string, prefix string) string {
	for key, path := range sources {
		if strings.HasPrefix(key, prefix) {
			return filepath.Dir(path)
		}
	}
	return ""
}
