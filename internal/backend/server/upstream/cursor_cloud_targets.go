package upstream

import (
	"fmt"
	"net/url"
	"strings"
)

var cursorCloudUpstreamTargets = map[string]string{
	"/aiserver.v1.AiService/StreamCpp":                            "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamCpp",
	"/aiserver.v1.AiService/StreamNextCursorPrediction":           "https://api4.cursor.sh:443/aiserver.v1.AiService/StreamNextCursorPrediction",
	"/aiserver.v1.AiService/GetCppEditClassification":             "https://api4.cursor.sh:443/aiserver.v1.AiService/GetCppEditClassification",
	"/aiserver.v1.AiService/RefreshTabContext":                    "https://api2.cursor.sh:443/aiserver.v1.AiService/RefreshTabContext",
	"/aiserver.v1.AiService/CppConfig":                            "https://api4.cursor.sh:443/aiserver.v1.AiService/CppConfig",
	"/aiserver.v1.AiService/CppEditHistoryStatus":                 "https://api2.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryStatus",
	"/aiserver.v1.AiService/CppAppend":                            "https://api3.cursor.sh:443/aiserver.v1.AiService/CppAppend",
	"/aiserver.v1.AiService/CppEditHistoryAppend":                 "https://api3.cursor.sh:443/aiserver.v1.AiService/CppEditHistoryAppend",
	"/aiserver.v1.CppService/AvailableModels":                     "https://api3.cursor.sh:443/aiserver.v1.CppService/AvailableModels",
	"/aiserver.v1.CppService/RecordCppFate":                       "https://api2.cursor.sh:443/aiserver.v1.CppService/RecordCppFate",
	"/aiserver.v1.AiService/ReportAiCodeChangeMetrics":            "https://api2.cursor.sh:443/aiserver.v1.AiService/ReportAiCodeChangeMetrics",
	"/aiserver.v1.AiService/WriteGitCommitMessage":                "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitCommitMessage",
	"/aiserver.v1.AiService/WriteGitBranchName":                   "https://api2.cursor.sh:443/aiserver.v1.AiService/WriteGitBranchName",
	"/aiserver.v1.FileSyncService/FSSyncFile":                     "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSSyncFile",
	"/aiserver.v1.FileSyncService/FSIsEnabledForUser":             "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSIsEnabledForUser",
	"/aiserver.v1.FileSyncService/FSConfig":                       "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSConfig",
	"/aiserver.v1.FileSyncService/FSUploadFile":                   "https://api4.cursor.sh:443/aiserver.v1.FileSyncService/FSUploadFile",
	"/aiserver.v1.DashboardService/GetEffectiveUserPlugins":       "https://api2.cursor.sh:443/aiserver.v1.DashboardService/GetEffectiveUserPlugins",
	"/aiserver.v1.DashboardService/ListMarketplaces":              "https://api2.cursor.sh:443/aiserver.v1.DashboardService/ListMarketplaces",
	"/aiserver.v1.DashboardService/AddMarketplace":                "https://api2.cursor.sh:443/aiserver.v1.DashboardService/AddMarketplace",
	"/aiserver.v1.DashboardService/RemoveMarketplace":             "https://api2.cursor.sh:443/aiserver.v1.DashboardService/RemoveMarketplace",
	"/aiserver.v1.DashboardService/UpdateMarketplace":             "https://api2.cursor.sh:443/aiserver.v1.DashboardService/UpdateMarketplace",
	"/aiserver.v1.DashboardService/RefreshMarketplace":            "https://api2.cursor.sh:443/aiserver.v1.DashboardService/RefreshMarketplace",
	"/aiserver.v1.DashboardService/RegisterMarketplaceAndPlugins": "https://api2.cursor.sh:443/aiserver.v1.DashboardService/RegisterMarketplaceAndPlugins",
	"/aiserver.v1.DashboardService/ListMarketplacePlugins":        "https://api2.cursor.sh:443/aiserver.v1.DashboardService/ListMarketplacePlugins",
	"/aiserver.v1.DashboardService/ResolvePluginsByRef":           "https://api2.cursor.sh:443/aiserver.v1.DashboardService/ResolvePluginsByRef",
	"/aiserver.v1.DashboardService/GetPlugin":                     "https://api2.cursor.sh:443/aiserver.v1.DashboardService/GetPlugin",
	"/aiserver.v1.DashboardService/ListUserPluginInstalls":        "https://api2.cursor.sh:443/aiserver.v1.DashboardService/ListUserPluginInstalls",
	"/aiserver.v1.DashboardService/InstallUserPlugin":             "https://api2.cursor.sh:443/aiserver.v1.DashboardService/InstallUserPlugin",
	"/aiserver.v1.DashboardService/UpdateUserPluginInstall":       "https://api2.cursor.sh:443/aiserver.v1.DashboardService/UpdateUserPluginInstall",
	"/aiserver.v1.DashboardService/UninstallUserPlugin":           "https://api2.cursor.sh:443/aiserver.v1.DashboardService/UninstallUserPlugin",
	"/aiserver.v1.DashboardService/ListTeamPluginInstalls":        "https://api2.cursor.sh:443/aiserver.v1.DashboardService/ListTeamPluginInstalls",
	"/aiserver.v1.DashboardService/InstallTeamPlugin":             "https://api2.cursor.sh:443/aiserver.v1.DashboardService/InstallTeamPlugin",
	"/aiserver.v1.DashboardService/UninstallTeamPlugin":           "https://api2.cursor.sh:443/aiserver.v1.DashboardService/UninstallTeamPlugin",
	"/aiserver.v1.DashboardService/GetPluginMcpConfig":            "https://api2.cursor.sh:443/aiserver.v1.DashboardService/GetPluginMcpConfig",
	"/aiserver.v1.DashboardService/AddMcpServersFromPlugin":       "https://api2.cursor.sh:443/aiserver.v1.DashboardService/AddMcpServersFromPlugin",
}

func ResolveCursorCloudTarget(path string, defaultHost string) (*url.URL, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return nil, fmt.Errorf("empty cursor cloud path")
	}
	rawTarget, ok := cursorCloudUpstreamTargets[normalizedPath]
	if !ok {
		host := strings.TrimSpace(defaultHost)
		if host == "" {
			host = "api2.cursor.sh:443"
		}
		rawTarget = "https://" + host + normalizedPath
	}
	parsed, err := url.Parse(rawTarget)
	if err != nil {
		return nil, fmt.Errorf("parse cursor cloud target failed: %w", err)
	}
	return parsed, nil
}
