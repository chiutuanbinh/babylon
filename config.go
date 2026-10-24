package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type config struct {
	Addr               string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	BaseURL            string
	AdminGitHubIDs     []int64
}

// loadConfig reads the environment through getenv (os.Getenv in main) so tests
// can supply their own. Every value is whitespace-trimmed. Every missing
// required variable is named in one error; malformed values fail at boot.
func loadConfig(rawGetenv func(string) string) (config, error) {
	getenv := func(k string) string { return strings.TrimSpace(rawGetenv(k)) }
	c := config{
		Addr:               ":8080",
		DatabaseURL:        getenv("DATABASE_URL"),
		GitHubClientID:     getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: getenv("GITHUB_CLIENT_SECRET"),
		BaseURL:            strings.TrimRight(getenv("BASE_URL"), "/"),
	}
	if v := getenv("ADDR"); v != "" {
		c.Addr = v
	}

	var missing []string
	for _, kv := range []struct{ name, val string }{
		{"DATABASE_URL", c.DatabaseURL},
		{"GITHUB_CLIENT_ID", c.GitHubClientID},
		{"GITHUB_CLIENT_SECRET", c.GitHubClientSecret},
		{"BASE_URL", c.BaseURL},
	} {
		if kv.val == "" {
			missing = append(missing, kv.name)
		}
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return config{}, fmt.Errorf("BASE_URL must be an absolute http(s) URL, got %q", c.BaseURL)
	}

	ids, err := parseGitHubIDs(getenv("ADMIN_GITHUB_IDS"))
	if err != nil {
		return config{}, err
	}
	c.AdminGitHubIDs = ids
	return c, nil
}

// parseGitHubIDs parses a comma-separated list of positive integers. Blank
// entries are skipped; anything else invalid is an error so a typo can never
// silently drop an admin.
func parseGitHubIDs(csv string) ([]int64, error) {
	var out []int64
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("ADMIN_GITHUB_IDS: %q is not a positive integer GitHub user ID", s)
		}
		out = append(out, id)
	}
	return out, nil
}

// secureCookies is false only for plain-http local development, where browsers
// would drop Secure cookies. The host must be exactly localhost or 127.0.0.1.
func (c config) secureCookies() bool {
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Scheme != "http" {
		return true
	}
	h := u.Hostname()
	return h != "localhost" && h != "127.0.0.1"
}
