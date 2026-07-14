package config

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"cursor/internal/cursor"
	"cursor/internal/logger"
	"cursor/internal/netproxy"
)

const (
	cursorOAuthTokenURL        = "https://api2.cursor.sh/oauth/token"
	cursorOAuthClientID        = "KbZUR41cY7W6zRSdpSUJ7I7mLYBKOCmB"
	cursorTokenRefreshLeadTime = 7 * 24 * time.Hour
)

type cursorOAuthTokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

type cursorOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ShouldLogout bool   `json:"shouldLogout"`
}

// CursorTokenRefresher keeps cursor.accessToken fresh using cursor.refreshToken.
type CursorTokenRefresher struct {
	manager *Manager
	client  *http.Client
	mu      sync.Mutex
}

func NewCursorTokenRefresher(manager *Manager) *CursorTokenRefresher {
	if manager == nil {
		return nil
	}
	return &CursorTokenRefresher{
		manager: manager,
		client:  netproxy.NewHTTPClient(30 * time.Second),
	}
}

func (refresher *CursorTokenRefresher) HasRefreshCredentials() bool {
	if refresher == nil || refresher.manager == nil {
		return false
	}
	return strings.TrimSpace(refresher.manager.Current().Cursor.RefreshToken) != ""
}

// EffectiveAccessToken returns a non-expired access token, refreshing and persisting when needed.
func (refresher *CursorTokenRefresher) EffectiveAccessToken(ctx context.Context) string {
	if refresher == nil || refresher.manager == nil {
		return ""
	}
	cfg := refresher.manager.Current()
	accessToken := strings.TrimSpace(cfg.Cursor.AccessToken)
	if accessToken == "" {
		return ""
	}
	if !refresher.shouldRefresh(accessToken) {
		return accessToken
	}
	refreshed, err := refresher.refreshAndPersist(ctx)
	if err != nil {
		logger.Errorf("cursor access token refresh skipped err=%v", err)
		return accessToken
	}
	return refreshed
}

// ApplyOAuthTokenResponse persists tokens from /oauth/token upstream response and syncs BYOK state DB.
func (refresher *CursorTokenRefresher) ApplyOAuthTokenResponse(ctx context.Context, statusCode int, body []byte) error {
	if refresher == nil || refresher.manager == nil || statusCode < 200 || statusCode >= 300 {
		return nil
	}
	response, err := parseCursorOAuthTokenResponse(body)
	if err != nil {
		return err
	}
	accessToken := strings.TrimSpace(response.AccessToken)
	if accessToken == "" {
		return fmt.Errorf("oauth token response missing access_token")
	}
	refreshToken := strings.TrimSpace(response.RefreshToken)
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(refresher.manager.Current().Cursor.RefreshToken)
	}
	return refresher.persistTokens(ctx, accessToken, refreshToken)
}

func (refresher *CursorTokenRefresher) shouldRefresh(accessToken string) bool {
	if !refresher.HasRefreshCredentials() {
		return false
	}
	expiresAt, ok := JWTPayloadExpiry(accessToken)
	if !ok {
		return true
	}
	return time.Until(expiresAt) <= cursorTokenRefreshLeadTime
}

func (refresher *CursorTokenRefresher) refreshAndPersist(ctx context.Context) (string, error) {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()

	cfg := refresher.manager.Current()
	accessToken := strings.TrimSpace(cfg.Cursor.AccessToken)
	if accessToken != "" && !refresher.shouldRefresh(accessToken) {
		return accessToken, nil
	}
	refreshToken := strings.TrimSpace(cfg.Cursor.RefreshToken)
	if refreshToken == "" {
		return accessToken, fmt.Errorf("cursor.refreshToken 未配置")
	}

	response, err := requestCursorOAuthToken(ctx, refresher.client, refreshToken)
	if err != nil {
		return accessToken, err
	}
	newAccessToken := strings.TrimSpace(response.AccessToken)
	if newAccessToken == "" {
		return accessToken, fmt.Errorf("cursor oauth refresh returned empty access_token")
	}
	newRefreshToken := strings.TrimSpace(response.RefreshToken)
	if newRefreshToken == "" {
		newRefreshToken = refreshToken
	}
	if err := refresher.persistTokens(ctx, newAccessToken, newRefreshToken); err != nil {
		return accessToken, err
	}
	return newAccessToken, nil
}

func (refresher *CursorTokenRefresher) persistTokens(ctx context.Context, accessToken, refreshToken string) error {
	current := refresher.manager.Current()
	current.Cursor.AccessToken = strings.TrimSpace(accessToken)
	if strings.TrimSpace(refreshToken) != "" {
		current.Cursor.RefreshToken = strings.TrimSpace(refreshToken)
	}
	if _, err := refresher.manager.Save(ctx, current); err != nil {
		return err
	}
	if err := syncCursorAuthInjection(current); err != nil {
		logger.Errorf("sync cursor auth injection after token refresh failed err=%v", err)
	}
	return nil
}

func requestCursorOAuthToken(ctx context.Context, client *http.Client, refreshToken string) (cursorOAuthTokenResponse, error) {
	if client == nil {
		client = netproxy.NewHTTPClient(30 * time.Second)
	}
	payload, err := json.Marshal(cursorOAuthTokenRequest{
		GrantType:    "refresh_token",
		ClientID:     cursorOAuthClientID,
		RefreshToken: strings.TrimSpace(refreshToken),
	})
	if err != nil {
		return cursorOAuthTokenResponse{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, cursorOAuthTokenURL, bytes.NewReader(payload))
	if err != nil {
		return cursorOAuthTokenResponse{}, err
	}
	request.Header.Set("content-type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return cursorOAuthTokenResponse{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return cursorOAuthTokenResponse{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return cursorOAuthTokenResponse{}, fmt.Errorf("cursor oauth refresh status=%d body=%s", response.StatusCode, truncateForLog(string(body), 256))
	}
	return parseCursorOAuthTokenResponse(body)
}

func parseCursorOAuthTokenResponse(body []byte) (cursorOAuthTokenResponse, error) {
	var response cursorOAuthTokenResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return cursorOAuthTokenResponse{}, fmt.Errorf("parse oauth token response failed: %w", err)
	}
	if response.ShouldLogout {
		return cursorOAuthTokenResponse{}, fmt.Errorf("cursor oauth refresh requested logout")
	}
	return response, nil
}

// JWTPayloadExpiry returns JWT exp claim when present.
func JWTPayloadExpiry(token string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func syncCursorAuthInjection(cfg Config) error {
	accessToken := strings.TrimSpace(cfg.Cursor.AccessToken)
	if accessToken == "" {
		return nil
	}
	email := strings.TrimSpace(cfg.Cursor.Email)
	if email == "" {
		email = "cursor@ai.com"
	}
	profile, err := cursor.ResolveRuntimeProfile(cfg.Cursor.UserDataDir)
	if err != nil {
		return err
	}
	return cursor.InjectCursorUserInfo(profile, email, accessToken, strings.TrimSpace(cfg.Cursor.RefreshToken))
}

func truncateForLog(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func CursorOAuthTokenURL() string {
	return cursorOAuthTokenURL
}

func MarshalCursorOAuthRefreshRequest(refreshToken string) ([]byte, error) {
	return json.Marshal(cursorOAuthTokenRequest{
		GrantType:    "refresh_token",
		ClientID:     cursorOAuthClientID,
		RefreshToken: strings.TrimSpace(refreshToken),
	})
}
