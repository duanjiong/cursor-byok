package upstream

import (
	"io"
	"net/http"
	"net/url"

	serverconfig "cursor/internal/backend/server/config"
	"cursor/internal/backend/server"
	"cursor/internal/netproxy"
)

func OAuthTokenAction(
	deps Dependencies,
	configs *serverconfig.Manager,
) server.HandlerFunc {
	return func(ctx *server.Context) error {
		if configs == nil {
			reqCtx, route, err := newCompatRouteObjects(ctx, deps, CompatRouteConfig{
				Name:       "oauth_token",
				StatusCode: http.StatusOK,
			})
			if err != nil {
				return err
			}
			return handleMockOAuth(reqCtx, route)
		}
		refresher := configs.CursorTokens()
		if refresher == nil || !refresher.HasRefreshCredentials() {
			reqCtx, route, err := newCompatRouteObjects(ctx, deps, CompatRouteConfig{
				Name:       "oauth_token",
				StatusCode: http.StatusOK,
			})
			if err != nil {
				return err
			}
			return handleMockOAuth(reqCtx, route)
		}

		reqCtx, route, err := newCompatRouteObjects(ctx, deps, CompatRouteConfig{Name: "oauth_token"})
		if err != nil {
			return err
		}
		_ = route
		targetURL, err := url.Parse(serverconfig.CursorOAuthTokenURL())
		if err != nil {
			return err
		}
		reqCtx.TargetURL = targetURL

		requestBody := reqCtx.RequestBody
		if len(requestBody) == 0 {
			requestBody, err = serverconfig.MarshalCursorOAuthRefreshRequest(configs.Current().Cursor.RefreshToken)
			if err != nil {
				return err
			}
		}

		upstreamClient := deps.HTTPClient
		if upstreamClient == nil {
			upstreamClient = netproxy.NewHTTPClient(0)
		}
		upstreamRequest, upstreamClient, err := buildUpstreamRequest(reqCtx, requestBody, ForwardOptions{})
		if err != nil {
			return err
		}
		upstreamRequest.Header.Set("content-type", "application/json")

		response, err := upstreamClient.Do(upstreamRequest)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		_ = refresher.ApplyOAuthTokenResponse(ctx.Request.Context(), response.StatusCode, responseBody)

		copyResponseHeadersToClient(reqCtx.ResponseWriter.Header(), response.Header)
		reqCtx.ResponseWriter.WriteHeader(response.StatusCode)
		_, err = reqCtx.ResponseWriter.Write(responseBody)
		return err
	}
}
