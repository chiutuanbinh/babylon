package main

import (
	"fmt"
	"strings"
)

type config struct {
	Addr               string
	DatabaseURL        string
	GitHubClientID     string
	GitHubClientSecret string
	BaseURL            string
	AdminLogins        []string
}

// loadConfig reads the environment through getenv (os.Getenv in main) so tests
// can supply their own. Every missing required variable is named in one error.
func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		Addr:               ":8080",
		DatabaseURL:        getenv("DATABASE_URL"),
		GitHubClientID:     getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: getenv("GITHUB_CLIENT_SECRET"),
		BaseURL:            strings.TrimRight(getenv("BASE_URL"), "/"),
		AdminLogins:        splitLogins(getenv("ADMIN_LOGINS")),
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
	return c, nil
}

func splitLogins(csv string) []string {
	var out []string
	for _, s := range strings.Split(csv, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// secureCookies is false only for plain-http local development, where browsers
// would drop Secure cookies.
func (c config) secureCookies() bool {
	return !(strings.HasPrefix(c.BaseURL, "http://localhost") ||
		strings.HasPrefix(c.BaseURL, "http://127.0.0.1"))
}
