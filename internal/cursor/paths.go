package cursor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const envBYOKUserDataDir = "CURSOR_BYOK_USER_DATA_DIR"

// RuntimeProfile 表示 Cursor --user-data-dir 根目录及其派生路径。
type RuntimeProfile struct {
	UserDataDir string
}

// DefaultBYOKUserDataDir 返回与 setup-cursor-byok-ide.sh 一致的 BYOK 专用数据目录。
func DefaultBYOKUserDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %w", err)
	}

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", "Cursor-BYOK"), nil
	case "windows":
		appData := strings.TrimSpace(os.Getenv("APPDATA"))
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		return filepath.Join(appData, "Cursor-BYOK"), nil
	case "linux":
		configDir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
		if configDir == "" {
			configDir = filepath.Join(homeDir, ".config")
		}
		return filepath.Join(configDir, "Cursor-BYOK"), nil
	default:
		return "", fmt.Errorf("不支持的系统: %s", runtime.GOOS)
	}
}

// ResolveRuntimeProfile 根据配置、环境变量或 BYOK 默认值构造运行时 profile。
func ResolveRuntimeProfile(configUserDataDir string) (RuntimeProfile, error) {
	dir := strings.TrimSpace(configUserDataDir)
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv(envBYOKUserDataDir))
	}
	if dir == "" {
		defaultDir, err := DefaultBYOKUserDataDir()
		if err != nil {
			return RuntimeProfile{}, err
		}
		dir = defaultDir
	}

	expanded, err := ExpandUserDataDir(dir)
	if err != nil {
		return RuntimeProfile{}, err
	}
	return RuntimeProfile{UserDataDir: expanded}, nil
}

// ExpandUserDataDir 展开 ~ 前缀并规范化路径。
func ExpandUserDataDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("Cursor 用户数据目录为空")
	}
	if strings.HasPrefix(dir, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("展开 Cursor 用户数据目录失败: %w", err)
		}
		if dir == "~" {
			dir = homeDir
		} else if strings.HasPrefix(dir, "~/") || strings.HasPrefix(dir, "~\\") {
			dir = filepath.Join(homeDir, dir[2:])
		} else {
			return "", fmt.Errorf("不支持的 Cursor 用户数据目录: %s", dir)
		}
	}
	return filepath.Clean(dir), nil
}

// SettingsPath 返回 profile 对应的 settings.json 路径。
func (profile RuntimeProfile) SettingsPath() (string, error) {
	root, err := profile.resolvedUserDataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "User", "settings.json"), nil
}

// StateDBPath 返回 profile 对应的 state.vscdb 路径。
func (profile RuntimeProfile) StateDBPath() (string, error) {
	root, err := profile.resolvedUserDataRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "User", "globalStorage", "state.vscdb"), nil
}

func (profile RuntimeProfile) resolvedUserDataRoot() (string, error) {
	dir := strings.TrimSpace(profile.UserDataDir)
	if dir == "" {
		return "", fmt.Errorf("Cursor 用户数据目录未配置")
	}
	return filepath.Clean(dir), nil
}
