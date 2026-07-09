package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Store struct {
	path     string
	logsRoot string
	mu       sync.Mutex
}

type fileSnapshot struct {
	exists  bool
	modTime int64
	size    int64
}

func NewStore(path string, logsRoot string) *Store {
	return &Store{
		path:     strings.TrimSpace(path),
		logsRoot: strings.TrimSpace(logsRoot),
	}
}

func (store *Store) Path() string {
	if store == nil {
		return ""
	}
	return store.path
}

func (store *Store) LogsRoot() string {
	if store == nil {
		return ""
	}
	return store.logsRoot
}

func (store *Store) snapshot() fileSnapshot {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return fileSnapshot{}
	}
	info, err := os.Stat(store.path)
	if err != nil {
		return fileSnapshot{}
	}
	return fileSnapshot{
		exists:  true,
		modTime: info.ModTime().UnixNano(),
		size:    info.Size(),
	}
}

func (store *Store) Load(_ context.Context) (Config, error) {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return DefaultConfig(), nil
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	data, err := os.ReadFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			defaultConfig := DefaultConfig()
			if err := store.saveLocked(defaultConfig); err != nil {
				return DefaultConfig(), err
			}
			return defaultConfig, nil
		}
		return DefaultConfig(), fmt.Errorf("读取用户配置失败: %w", err)
	}

	var current Config
	if err := yaml.Unmarshal(data, &current); err != nil {
		return DefaultConfig(), fmt.Errorf("解析用户配置失败: %w", err)
	}
	normalized, err := NormalizeConfig(current)
	if err != nil {
		return DefaultConfig(), err
	}
	if shouldPersistNormalizedConfig(data, current, normalized) {
		if err := store.saveLocked(normalized); err != nil {
			return DefaultConfig(), err
		}
	}
	return normalized, nil
}

func (store *Store) Save(_ context.Context, cfg Config) (Config, error) {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return Config{}, errors.New("配置存储未初始化")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	merged := cfg
	if existing, err := store.loadLocked(); err == nil {
		if strings.TrimSpace(cfg.TabServerBaseURL) == "" && strings.TrimSpace(existing.TabServerBaseURL) != "" {
			merged.TabServerBaseURL = existing.TabServerBaseURL
		}
	}

	normalized, err := NormalizeConfig(merged)
	if err != nil {
		return Config{}, err
	}

	if err := store.saveLocked(normalized); err != nil {
		return Config{}, err
	}
	return normalized, nil
}

func (store *Store) loadLocked() (Config, error) {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return DefaultConfig(), nil
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("读取用户配置失败: %w", err)
	}
	var current Config
	if err := yaml.Unmarshal(data, &current); err != nil {
		return Config{}, fmt.Errorf("解析用户配置失败: %w", err)
	}
	return NormalizeConfig(current)
}

func (store *Store) saveLocked(normalized Config) error {
	writePath, err := resolveConfigWritePath(store.path)
	if err != nil {
		return fmt.Errorf("解析配置写入路径失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(writePath), 0o755); err != nil {
		return fmt.Errorf("创建用户配置目录失败: %w", err)
	}

	data, err := yaml.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("序列化用户配置失败: %w", err)
	}

	tempPath := writePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("写入临时配置失败: %w", err)
	}
	if err := os.Rename(tempPath, writePath); err != nil {
		return fmt.Errorf("保存用户配置失败: %w", err)
	}
	return nil
}

// resolveConfigWritePath 返回实际应写入的配置文件路径。
// 当 store.path 是软链接时，写入链接目标文件，避免 rename 覆盖软链接本身。
func resolveConfigWritePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("配置路径为空")
	}

	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return path, nil
		}
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return filepath.Clean(target), nil
}

func shouldPersistNormalizedConfig(raw []byte, current Config, normalized Config) bool {
	if !yamlHasKey(raw, "backendListenAddr") || !yamlHasKey(raw, "proxyListenAddr") {
		return true
	}
	if !yamlHasKey(raw, "cursor") || strings.TrimSpace(current.Cursor.UserDataDir) != strings.TrimSpace(normalized.Cursor.UserDataDir) {
		return true
	}
	if current.BackendListenAddr != normalized.BackendListenAddr || current.ProxyListenAddr != normalized.ProxyListenAddr {
		return true
	}
	if current.ProviderStreamIdleTimeout == normalized.ProviderStreamIdleTimeout {
		return false
	}
	return yamlHasKey(raw, "providerStreamIdleTimeout")
}

func yamlHasKey(raw []byte, key string) bool {
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return false
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return false
	}
	mapping := root.Content[0]
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return true
		}
	}
	return false
}
