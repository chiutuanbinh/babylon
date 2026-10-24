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
	env["ADMIN_LOGINS"] = " root, ,Alice ,"
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
	if want := []string{"root", "Alice"}; !reflect.DeepEqual(c.AdminLogins, want) {
		t.Fatalf("AdminLogins = %q, want %q", c.AdminLogins, want)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	c, err := loadConfig(envFrom(fullEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || len(c.AdminLogins) != 0 {
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

func TestSecureCookies(t *testing.T) {
	tests := map[string]bool{
		"http://localhost:8080": false,
		"http://127.0.0.1:8080": false,
		"https://example.com":   true,
		"http://example.com":    true,
	}
	for base, want := range tests {
		if got := (config{BaseURL: base}).secureCookies(); got != want {
			t.Errorf("secureCookies(%q) = %v, want %v", base, got, want)
		}
	}
}
