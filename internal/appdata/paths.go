package appdata

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	appDirName       = ".cursor-local-assistant-v2"
	legacyAppDirName = ".cursor-local-assistant"
)

// RootDir 返回应用配置根目录。
func RootDir() string {
	return appRootDir(appDirName)
}

func legacyRootDir() string {
	return appRootDir(legacyAppDirName)
}

func appRootDir(dirName string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return dirName
	}
	return filepath.Join(homeDir, dirName)
}

// ConfigFilePath 返回统一用户配置文件路径。
func ConfigFilePath() string {
	return filepath.Join(RootDir(), "config.yaml")
}

// LastAgentModelHashFilePath 返回最近一次 agent 渠道 hash 的运行时状态文件路径。
// 该值不属于用户配置，不写入 config.yaml。
func LastAgentModelHashFilePath() string {
	return filepath.Join(RootDir(), "last-agent-model-hash")
}

func DataRootPath() string {
	return filepath.Join(RootDir(), "data")
}

func HistoryRootPath() string {
	return filepath.Join(RootDir(), "history")
}

func UsageFilePath() string {
	return filepath.Join(HistoryRootPath(), "usage.json")
}

func AdsRootPath() string {
	return filepath.Join(DataRootPath(), "ads")
}

func CodebaseIndexRootPath() string {
	return filepath.Join(DataRootPath(), "codebase-index")
}

func DocsIndexRootPath() string {
	return filepath.Join(DataRootPath(), "docs-index")
}

func RulesRootPath() string {
	return filepath.Join(RootDir(), "rules")
}

// SkillsConfigFilePath 返回全局 skills 配置文件路径（用户级覆盖）。
func SkillsConfigFilePath() string {
	return filepath.Join(RootDir(), "skills.yaml")
}

// SkillsCacheRootPath 返回远程 skill 拉取缓存根目录。
func SkillsCacheRootPath() string {
	return filepath.Join(DataRootPath(), "skills-cache")
}

// LogsRootPath 返回统一日志根目录路径。
func LogsRootPath() string {
	return filepath.Join(RootDir(), "logs")
}

// CACertFilePath 返回注入给宿主的 CA 文件路径。
func CACertFilePath() string {
	return filepath.Join(DataRootPath(), "ca.crt")
}
