package skillsconfig

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var githubRefPattern = regexp.MustCompile(`^(?:(?:https?://github\.com/)|(?:github:)?)([^/]+)/([^/@#]+)(?:/(.*))?(?:@([^@#]+))?$`)

type githubSource struct {
	Owner   string
	Repo    string
	Subpath string
	Ref     string
}

func parseGitHubSource(raw string) (githubSource, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "github:")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return githubSource{}, fmt.Errorf("github source is empty")
	}

	if strings.Contains(raw, "github.com/") {
		return parseGitHubURL(raw)
	}

	matches := githubRefPattern.FindStringSubmatch(raw)
	if len(matches) == 0 {
		return githubSource{}, fmt.Errorf("invalid github source %q", raw)
	}

	ref := strings.TrimSpace(matches[4])
	if ref == "" {
		ref = "main"
	}
	subpath := strings.Trim(strings.TrimSpace(matches[3]), "/")
	return githubSource{
		Owner:   strings.TrimSpace(matches[1]),
		Repo:    strings.TrimSpace(matches[2]),
		Subpath: subpath,
		Ref:     ref,
	}, nil
}

func parseGitHubURL(raw string) (githubSource, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	trimmed = strings.TrimPrefix(trimmed, "github.com/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		return githubSource{}, fmt.Errorf("invalid github url %q", raw)
	}
	owner := strings.TrimSpace(parts[0])
	repo := strings.TrimSpace(parts[1])
	ref := "main"
	subpath := ""
	if len(parts) >= 4 && parts[2] == "tree" {
		ref = strings.TrimSpace(parts[3])
		if len(parts) > 4 {
			subpath = strings.Join(parts[4:], "/")
		}
	} else if len(parts) > 2 {
		subpath = strings.Join(parts[2:], "/")
		if at := strings.LastIndex(subpath, "@"); at > 0 {
			ref = strings.TrimSpace(subpath[at+1:])
			subpath = strings.TrimSpace(subpath[:at])
		}
	}
	if ref == "" {
		ref = "main"
	}
	return githubSource{
		Owner:   owner,
		Repo:    repo,
		Subpath: strings.Trim(subpath, "/"),
		Ref:     ref,
	}, nil
}

func githubCacheDir(cacheRoot string, source githubSource) string {
	parts := []string{
		cacheRoot,
		"github",
		source.Owner,
		source.Repo,
		sanitizePathSegment(source.Ref),
	}
	if source.Subpath != "" {
		parts = append(parts, splitPathSegments(source.Subpath)...)
	}
	return filepath.Join(parts...)
}

func syncGitHubSource(cacheRoot string, raw string) (string, error) {
	source, err := parseGitHubSource(raw)
	if err != nil {
		return "", err
	}
	cacheDir := githubCacheDir(cacheRoot, source)
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		return "", fmt.Errorf("create github skills cache parent: %w", err)
	}

	if _, err := os.Stat(filepath.Join(cacheDir, ".git")); err == nil {
		if err := runGit(cacheDir, "fetch", "--depth", "1", "origin", source.Ref); err != nil {
			return "", err
		}
		if err := runGit(cacheDir, "checkout", "FETCH_HEAD"); err != nil {
			return "", err
		}
		return cacheDir, nil
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return "", fmt.Errorf("reset github skills cache: %w", err)
	}
	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", source.Owner, source.Repo)
	if err := runGit("", "clone", "--depth", "1", "--branch", source.Ref, "--single-branch", cloneURL, cacheDir); err != nil {
		return "", err
	}
	if source.Subpath != "" {
		nested := filepath.Join(cacheDir, filepath.FromSlash(source.Subpath))
		if _, err := os.Stat(filepath.Join(nested, "SKILL.md")); err != nil {
			return "", fmt.Errorf("github skill missing SKILL.md at %s", nested)
		}
		return nested, nil
	}
	return cacheDir, nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_")
	return replacer.Replace(value)
}

func splitPathSegments(path string) []string {
	path = strings.Trim(path, "/\\")
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = sanitizePathSegment(part)
		if part != "" && part != "_" {
			segments = append(segments, part)
		}
	}
	return segments
}
