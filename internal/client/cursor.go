package client

import (
	"fmt"
	goruntime "runtime"

	"cursor/internal/cursor"
)

func (s *ProxyService) cursorRuntimeProfile() (cursor.RuntimeProfile, error) {
	cfg, err := s.LoadUserConfig()
	if err != nil {
		return cursor.RuntimeProfile{}, err
	}
	return cursor.ResolveRuntimeProfile(cfg.Cursor.UserDataDir)
}

// ApplyCursorSettings 用于处理与 ApplyCursorSettings 相关的逻辑。
func (s *ProxyService) ApplyCursorSettings() error {
	if s == nil || s.proxy == nil {
		return fmt.Errorf("proxy is not initialized")
	}
	profile, err := s.cursorRuntimeProfile()
	if err != nil {
		return fmt.Errorf("resolve cursor profile: %w", err)
	}
	s.caFileMu.Lock()
	caCertPath, err := cursor.EnsureCACertFile(s.caCertPEM, s.caFilePath)
	if err == nil {
		s.caFilePath = caCertPath
	}
	s.caFileMu.Unlock()
	if err != nil {
		return fmt.Errorf("ensure ca cert file: %w", err)
	}

	switch goruntime.GOOS {
	case "windows":
		if err := cursor.EnsureCACertInstalled(s.caCertPEM, caCertPath); err != nil {
			return fmt.Errorf("install ca cert: %w", err)
		}
	case "darwin":
		if err := cursor.EnsureCACertInstalled(s.caCertPEM, caCertPath); err != nil {
			return fmt.Errorf("install ca cert: %w", err)
		}
		if err := cursor.SetSystemNodeExtraCACerts(caCertPath); err != nil {
			return fmt.Errorf("set node extra ca certs: %w", err)
		}
	}

	if err := cursor.WriteUserProxySettings(profile, cursor.ProxyURLFromListenAddr(s.proxy.Snapshot().ListenAddr)); err != nil {
		return err
	}
	s.setCursorSettingsApplied(true)
	return nil
}

// ClearCursorSettings 用于处理与 ClearCursorSettings 相关的逻辑。
func (s *ProxyService) ClearCursorSettings() error {
	profile, err := s.cursorRuntimeProfile()
	if err != nil {
		return fmt.Errorf("resolve cursor profile: %w", err)
	}
	if goruntime.GOOS == "darwin" {
		if err := cursor.ClearSystemNodeExtraCACerts(); err != nil {
			return err
		}
	}
	if err := cursor.ClearUserProxySettings(profile); err != nil {
		return err
	}
	s.setCursorSettingsApplied(false)
	return nil
}

// GetDeviceID 用于处理与 GetDeviceID 相关的逻辑。
func (s *ProxyService) GetDeviceID() (string, error) {
	return cursor.GetDeviceID()
}
