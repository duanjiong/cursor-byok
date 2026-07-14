package upstream

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	serverconfig "cursor/internal/backend/server/config"
	"cursor/internal/backend/server"
)

type CompatRouteConfig struct {
	Name                        string
	StatusCode                  int
	JSONBody                    map[string]any
	MockProtoType               string
	MockBuilder                 func(*RequestContext) (map[string]any, error)
	ConsoleLog                  bool
	PreserveClientAuthorization bool
	ForceAuthorizationToken     string
	ForceCookie                 string
}

func DirectAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleDirect(reqCtx, route)
	}
}

type CursorCloudRouteOptions struct {
	DefaultHost               string
	PreserveClientWhenNoToken bool
	AllowLegacyTabServer      bool
}

func CursorCloudDirectAction(
	deps Dependencies,
	configs *serverconfig.Manager,
	tokenRefresher *serverconfig.CursorTokenRefresher,
	cfg CompatRouteConfig,
	opts CursorCloudRouteOptions,
) server.HandlerFunc {
	return func(ctx *server.Context) error {
		if ctx == nil {
			return nil
		}
		routeCfg := cfg
		cloudCfg := serverconfig.DefaultConfig().Cursor
		tabServerBaseURL := ""
		if configs != nil {
			current := configs.Current()
			cloudCfg = current.Cursor
			tabServerBaseURL = strings.TrimSpace(current.TabServerBaseURL)
		}
		token := strings.TrimSpace(cloudCfg.AccessToken)
		if tokenRefresher != nil && tokenRefresher.HasRefreshCredentials() {
			if refreshed := strings.TrimSpace(tokenRefresher.EffectiveAccessToken(ctx.Request.Context())); refreshed != "" {
				token = refreshed
			}
		}
		if token != "" {
			routeCfg.ForceAuthorizationToken = token
			routeCfg.ForceCookie = strings.TrimSpace(cloudCfg.Cookie)
		} else if opts.PreserveClientWhenNoToken {
			routeCfg.PreserveClientAuthorization = true
		}

		if ctx.Request != nil && ctx.Request.URL != nil {
			switch {
			case token != "" || !opts.AllowLegacyTabServer || tabServerBaseURL == "":
				target, err := ResolveCursorCloudTarget(ctx.Request.URL.Path, opts.DefaultHost)
				if err != nil {
					return err
				}
				targetURL := *target
				targetURL.RawQuery = ctx.Request.URL.RawQuery
				ctx.UpstreamURL = &targetURL
			case opts.AllowLegacyTabServer && tabServerBaseURL != "":
				baseURL, err := url.Parse(tabServerBaseURL)
				if err != nil {
					return err
				}
				targetURL := *ctx.Request.URL
				targetURL.Scheme = baseURL.Scheme
				targetURL.Host = baseURL.Host
				ctx.UpstreamURL = &targetURL
			}
		}

		reqCtx, route, err := newCompatRouteObjects(ctx, deps, routeCfg)
		if err != nil {
			return err
		}
		return handleDirect(reqCtx, route)
	}
}

func FixedStatusAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleFixedStatus(reqCtx, route)
	}
}

func MockJSONAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockJSON(reqCtx, route)
	}
}

func MockOAuthAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockOAuth(reqCtx, route)
	}
}

func MockAuthFullStripeProfileAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockAuthFullStripeProfile(reqCtx, route)
	}
}

func MockAuthStripeProfileAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockAuthStripeProfile(reqCtx, route)
	}
}

func MockAuthPollAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockAuthPoll(reqCtx, route)
	}
}

func MockAuthEmailAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockAuthEmail(reqCtx, route)
	}
}

func MockProtoAction(deps Dependencies, cfg CompatRouteConfig) server.HandlerFunc {
	return func(ctx *server.Context) error {
		reqCtx, route, err := newCompatRouteObjects(ctx, deps, cfg)
		if err != nil {
			return err
		}
		return handleMockProto(reqCtx, route)
	}
}

func newCompatRouteObjects(ctx *server.Context, deps Dependencies, cfg CompatRouteConfig) (*RequestContext, *Route, error) {
	if ctx == nil || ctx.Request == nil {
		return nil, nil, nil
	}
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, nil, err
	}
	ctx.Request.Body = io.NopCloser(bytes.NewReader(body))
	targetURL := ctx.UpstreamURL
	if targetURL == nil && ctx.Request.URL != nil {
		copyURL := *ctx.Request.URL
		targetURL = &copyURL
	}
	reqCtx := &RequestContext{
		ResponseWriter: ctx.Writer,
		Request:        ctx.Request,
		StartedAt:      ctx.StartedAt,
		RawURL:         strings.TrimSpace(ctx.Request.Header.Get(server.HeaderServerUpstreamURL)),
		TargetURL:      targetURL,
		Method:         strings.ToUpper(strings.TrimSpace(ctx.Request.Method)),
		Headers:        ctx.Request.Header.Clone(),
		ContentType:    strings.TrimSpace(ctx.Request.Header.Get("content-type")),
		RequestBody:    body,
		Mode:           ctx.Mode,
		Deps:           &deps,
		HTTPRequestID:  resolveHTTPRequestID(ctx.Request),
	}
	route := &Route{
		Name:                        cfg.Name,
		Pattern:                     ctx.Request.URL.Path,
		StatusCode:                  cfg.StatusCode,
		JSONBody:                    cfg.JSONBody,
		MockProtoType:               cfg.MockProtoType,
		MockPayloadBuilder:          cfg.MockBuilder,
		ConsoleLog:                  cfg.ConsoleLog,
		PreserveClientAuthorization: cfg.PreserveClientAuthorization,
		ForceAuthorizationToken:     cfg.ForceAuthorizationToken,
		ForceCookie:                 cfg.ForceCookie,
	}
	return reqCtx, route, nil
}

func ServerTimeMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildServerTimePayload(reqCtx)
}

func ServerConfigMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildServerConfigPayload(reqCtx)
}

func AvailableModelsMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildAvailableModelsPayload(reqCtx)
}

func DefaultModelNudgeMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDefaultModelNudgeDataPayload(reqCtx)
}

func BootstrapStatsigMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildBootstrapStatsigPayload(reqCtx)
}

func FirstWindowStatsigDecisionMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildFirstWindowStatsigDecisionPayload(reqCtx)
}

func DashboardCurrentPeriodUsageMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardCurrentPeriodUsagePayload(reqCtx)
}

func DashboardTeamsMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardTeamsPayload(reqCtx)
}

func DashboardManagedSkillsMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardManagedSkillsPayload(reqCtx)
}

func DashboardGetMeMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardGetMePayload(reqCtx)
}

func DashboardUserPrivacyModeMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardUserPrivacyModePayload(reqCtx)
}

func DashboardPlanInfoMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardPlanInfoPayload(reqCtx)
}

func DashboardUsageLimitStatusAndActiveGrantsMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardUsageLimitStatusAndActiveGrantsPayload(reqCtx)
}

func DashboardIsOnNewPricingMockBuilder(reqCtx *RequestContext) (map[string]any, error) {
	return buildDashboardIsOnNewPricingPayload(reqCtx)
}

func resolveHTTPRequestID(request *http.Request) string {
	requestID := strings.TrimSpace(request.Header.Get("x-request-id"))
	if requestID != "" {
		return requestID
	}
	return strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "-")
}
