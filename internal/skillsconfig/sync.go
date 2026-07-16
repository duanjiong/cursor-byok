package skillsconfig

import (
	"fmt"
	"os"
	"strings"
)

// Sync 根据 skills.yaml 物化全局与项目 skill。
func Sync(workspaceRoot string) ([]ResolvedSkill, error) {
	resolver := NewResolver()
	skills, err := resolver.Resolve(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return skills, nil
}

// SyncFromEnv 读取环境变量中的 workspace 并同步 skill。
func SyncFromEnv() ([]ResolvedSkill, error) {
	workspaceRoot := strings.TrimSpace(os.Getenv("CURSOR_BYOK_WORKSPACE"))
	if workspaceRoot == "" {
		workspaceRoot, _ = os.Getwd()
	}
	return Sync(workspaceRoot)
}

// DescribeResolved 打印同步结果，供 CLI 使用。
func DescribeResolved(skills []ResolvedSkill) string {
	if len(skills) == 0 {
		return "no configured skills resolved"
	}
	lines := make([]string, 0, len(skills)+1)
	lines = append(lines, fmt.Sprintf("resolved %d configured skill(s):", len(skills)))
	for _, skill := range skills {
		lines = append(lines, fmt.Sprintf("- [%s] %s -> %s (%s)", skill.Scope, skill.Name, skill.ReadmePath, skill.Source))
	}
	return strings.Join(lines, "\n")
}
