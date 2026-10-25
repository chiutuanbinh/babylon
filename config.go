package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
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

// defaultConfigFile is read when CONFIG_FILE is unset; it may be absent.
const defaultConfigFile = ".env"

// readConfig is the boot entry point. Values come from the real environment
// (lookup, os.LookupEnv in main) first, then from a dotenv-format file whose
// path is CONFIG_FILE, default ".env". A missing default file is fine
// (production has none); a missing explicit CONFIG_FILE is an error.
func readConfig(lookup func(string) (string, bool)) (config, error) {
	path, explicit := lookup("CONFIG_FILE")
	if path = strings.TrimSpace(path); path == "" {
		path, explicit = defaultConfigFile, false
	}
	if explicit {
		if _, err := os.Stat(path); err != nil {
			return config{}, fmt.Errorf("CONFIG_FILE: %w", err)
		}
	}
	file, err := loadDotEnv(path)
	if err != nil {
		return config{}, err
	}
	return loadConfig(layeredGetenv(lookup, file))
}

// loadConfig reads settings through getenv (layeredGetenv in main) so tests
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

	if h := u.Hostname(); u.Scheme == "http" && h != "localhost" && h != "127.0.0.1" {
		return config{}, fmt.Errorf("BASE_URL %q uses plain http; use https (http is only allowed for localhost or 127.0.0.1)", c.BaseURL)
	}

	if getenv("ADMIN_LOGINS") != "" {
		return config{}, fmt.Errorf("ADMIN_LOGINS is no longer supported; set ADMIN_GITHUB_IDS to numeric GitHub user IDs")
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

// loadDotEnv reads KEY=VALUE pairs from a dotenv file. A missing file is not an
// error (production has none); an unreadable or malformed one is, so a typo
// fails at boot instead of silently dropping a setting.
//
// Supported: blank lines, "# comment" lines, an optional "export " prefix,
// whitespace around the key and value, "..." and '...' quoted values (no
// escapes), and a trailing " # comment" after an unquoted value. There is no
// variable expansion. Quote values that contain " #".
func loadDotEnv(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}

	env := map[string]string{}
	for i, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		key, raw, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("%s: line %d: want KEY=VALUE", path, i+1)
		}
		val, err := dotEnvValue(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("%s: line %d: %w", path, i+1, err)
		}
		env[key] = val
	}
	return env, nil
}

// dotEnvValue unquotes a value, or strips a trailing " #" comment from an
// unquoted one.
func dotEnvValue(raw string) (string, error) {
	if raw != "" && (raw[0] == '"' || raw[0] == '\'') {
		end := strings.IndexByte(raw[1:], raw[0])
		if end < 0 {
			return "", errors.New("unterminated quote")
		}
		if rest := strings.TrimSpace(raw[end+2:]); rest != "" && !strings.HasPrefix(rest, "#") {
			return "", errors.New("unexpected text after closing quote")
		}
		return raw[1 : end+1], nil
	}
	if i := strings.Index(raw, " #"); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(raw), nil
}

// layeredGetenv returns a getenv that prefers the real environment (lookup,
// normally os.LookupEnv) and falls back to the dotenv file. A variable that is
// set but empty in the real environment still wins, so it can blank out a file
// value.
func layeredGetenv(lookup func(string) (string, bool), file map[string]string) func(string) string {
	return func(key string) string {
		if v, ok := lookup(key); ok {
			return v
		}
		return file[key]
	}
}
