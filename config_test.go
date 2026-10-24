package main

import (
	"reflect"
	"strings"
	"testing"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func fullEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":         "postgres://u:p@h:5432/db",
		"GITHUB_CLIENT_ID":     "cid",
		"GITHUB_CLIENT_SECRET": "sec",
		"BASE_URL":             "http://localhost:8080/",
	}
}

func TestLoadConfig(t *testing.T) {
	env := fullEnv()
	env["ADMIN_GITHUB_IDS"] = " 12, ,34 ,"
	env["ADDR"] = ":9000"
	c, err := loadConfig(envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" || c.DatabaseURL != env["DATABASE_URL"] || c.GitHubClientID != "cid" || c.GitHubClientSecret != "sec" {
		t.Fatalf("config = %+v", c)
	}
	if c.BaseURL != "http://localhost:8080" {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", c.BaseURL)
	}
	if want := []int64{12, 34}; !reflect.DeepEqual(c.AdminGitHubIDs, want) {
		t.Fatalf("AdminGitHubIDs = %v, want %v", c.AdminGitHubIDs, want)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(envFrom(fullEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || len(c.AdminGitHubIDs) != 0 {
		t.Fatalf("config = %+v", c)
	}
}

func TestLoadConfigNamesEveryMissingVar(t *testing.T) {
	_, err := loadConfig(envFrom(map[string]string{"GITHUB_CLIENT_ID": "cid"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, name := range []string{"DATABASE_URL", "GITHUB_CLIENT_SECRET", "BASE_URL"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not mention %s", err, name)
		}
	}
	if strings.Contains(err.Error(), "GITHUB_CLIENT_ID") {
		t.Errorf("error %q mentions a variable that was set", err)
	}
}

func TestLoadConfigRejectsBadAdminIDs(t *testing.T) {
	for _, v := range []string{"alice", "12,bob", "0", "-5", "1.5", "99999999999999999999"} {
		env := fullEnv()
		env["ADMIN_GITHUB_IDS"] = v
		_, err := loadConfig(envFrom(env))
		if err == nil || !strings.Contains(err.Error(), "ADMIN_GITHUB_IDS") {
			t.Errorf("ADMIN_GITHUB_IDS=%q: err = %v, want error naming ADMIN_GITHUB_IDS", v, err)
		}
	}
}

func TestLoadConfigRejectsBadBaseURL(t *testing.T) {
	for _, v := range []string{"not a url", "ftp://x", "http://", "localhost:8080", "/relative"} {
		env := fullEnv()
		env["BASE_URL"] = v
		_, err := loadConfig(envFrom(env))
		if err == nil || !strings.Contains(err.Error(), "BASE_URL") {
			t.Errorf("BASE_URL=%q: err = %v, want error naming BASE_URL", v, err)
		}
	}
}

func TestLoadConfigTrimsWhitespace(t *testing.T) {
	env := fullEnv()
	for k, v := range env {
		env[k] = "  " + v + "\t"
	}
	c, err := loadConfig(envFrom(env))
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "postgres://u:p@h:5432/db" || c.GitHubClientID != "cid" || c.BaseURL != "http://localhost:8080" {
		t.Fatalf("config = %+v", c)
	}
}

func TestSecureCookies(t *testing.T) {
	tests := map[string]bool{
		"http://localhost:8080":          false,
		"http://localhost":               false,
		"http://127.0.0.1:8080":          false,
		"https://localhost":              true,
		"https://example.com":            true,
		"http://example.com":             true,
		"http://localhost.evil.com":      true,
		"http://127.0.0.1.evil.com":      true,
		"http://localhost@evil.com":      true,
		"http://localhost.evil.com:8080": true,
		// Failing safe is intended: anything not exactly localhost/127.0.0.1
		// gets Secure cookies, even if it is really a loopback address.
		"http://LOCALHOST:8080":  true,
		"http://localhost.:8080": true,
		"http://[::1]:8080":      true,
	}
	for base, want := range tests {
		if got := (config{BaseURL: base}).secureCookies(); got != want {
			t.Errorf("secureCookies(%q) = %v, want %v", base, got, want)
		}
	}
}

func TestLoadConfigRejectsPlainHTTPOutsideLoopback(t *testing.T) {
	for _, v := range []string{"http://example.com", "http://LOCALHOST:8080", "http://[::1]:8080", "http://localhost.evil.com"} {
		env := fullEnv()
		env["BASE_URL"] = v
		_, err := loadConfig(envFrom(env))
		if err == nil || !strings.Contains(err.Error(), "BASE_URL") || !strings.Contains(err.Error(), "https") {
			t.Errorf("BASE_URL=%q: err = %v, want error naming BASE_URL and https", v, err)
		}
	}
	for _, v := range []string{"https://example.com", "http://localhost:8080", "http://127.0.0.1:8080"} {
		env := fullEnv()
		env["BASE_URL"] = v
		if _, err := loadConfig(envFrom(env)); err != nil {
			t.Errorf("BASE_URL=%q: unexpected error %v", v, err)
		}
	}
}

func TestLoadConfigRejectsLegacyAdminLogins(t *testing.T) {
	env := fullEnv()
	env["ADMIN_LOGINS"] = "alice"
	_, err := loadConfig(envFrom(env))
	if err == nil || !strings.Contains(err.Error(), "ADMIN_LOGINS is no longer supported") {
		t.Fatalf("err = %v, want ADMIN_LOGINS unsupported error", err)
	}
}
