package main

import (
	"os"
	"path/filepath"
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

func writeEnvFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDotEnv(t *testing.T) {
	body := strings.Join([]string{
		"# comment",
		"",
		"PLAIN=value",
		"export EXPORTED=yes",
		"  SPACED  =  padded value  ",
		"EQUALS=a=b=c",
		"EMPTY=",
		`DOUBLE="has # hash and spaces"`,
		`SINGLE='single # quoted'`,
		"INLINE=value # trailing comment",
		"HASH_NO_SPACE=pa#ss",
		"CRLF=windows\r",
	}, "\n")
	got, err := loadDotEnv(writeEnvFile(t, body))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"PLAIN":         "value",
		"EXPORTED":      "yes",
		"SPACED":        "padded value",
		"EQUALS":        "a=b=c",
		"EMPTY":         "",
		"DOUBLE":        "has # hash and spaces",
		"SINGLE":        "single # quoted",
		"INLINE":        "value",
		"HASH_NO_SPACE": "pa#ss",
		"CRLF":          "windows",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadDotEnv = %v, want %v", got, want)
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	got, err := loadDotEnv(filepath.Join(t.TempDir(), "nope.env"))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want empty map and nil error", got, err)
	}
}

func TestLoadDotEnvUnreadableIsAnError(t *testing.T) {
	if _, err := loadDotEnv(t.TempDir()); err == nil { // a directory cannot be read as a file
		t.Fatal("want error reading a directory")
	}
}

func TestLoadDotEnvRejectsMalformedLines(t *testing.T) {
	tests := []struct{ name, body, wantErr string }{
		{"no equals", "A=1\nJUSTAKEY\n", "line 2"},
		{"empty key", "=value\n", "line 1"},
		{"key with space", "BAD KEY=1\n", "line 1"},
		{"unterminated double quote", `A="oops`, "line 1"},
		{"unterminated single quote", "ok=1\nA='oops", "line 2"},
		{"text after closing quote", `A="x" y`, "line 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadDotEnv(writeEnvFile(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestLayeredGetenv(t *testing.T) {
	process := map[string]string{"SET": "from-process", "SET_EMPTY": ""}
	lookup := func(k string) (string, bool) { v, ok := process[k]; return v, ok }
	file := map[string]string{"SET": "from-file", "SET_EMPTY": "from-file", "ONLY_FILE": "from-file"}

	get := layeredGetenv(lookup, file)
	for key, want := range map[string]string{
		"SET":       "from-process", // the real environment wins
		"SET_EMPTY": "",             // ...even when set but empty
		"ONLY_FILE": "from-file",    // the file fills gaps
		"NEITHER":   "",
	} {
		if got := get(key); got != want {
			t.Errorf("get(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestLoadConfigFromDotEnvFile(t *testing.T) {
	path := writeEnvFile(t, strings.Join([]string{
		"DATABASE_URL=postgres://u:p@h:5432/db",
		"GITHUB_CLIENT_ID=cid",
		"GITHUB_CLIENT_SECRET=sec",
		"BASE_URL=http://localhost:8080",
		"ADMIN_GITHUB_IDS=12,34",
	}, "\n"))
	file, err := loadDotEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	// An empty process environment: everything must come from the file.
	c, err := loadConfig(layeredGetenv(func(string) (string, bool) { return "", false }, file))
	if err != nil {
		t.Fatal(err)
	}
	if c.GitHubClientID != "cid" || c.BaseURL != "http://localhost:8080" || !reflect.DeepEqual(c.AdminGitHubIDs, []int64{12, 34}) {
		t.Fatalf("config = %+v", c)
	}
}

func TestReadConfigPrecedenceAndPath(t *testing.T) {
	path := writeEnvFile(t, strings.Join([]string{
		"DATABASE_URL=file-db",
		"GITHUB_CLIENT_ID=file-cid",
		"GITHUB_CLIENT_SECRET=sec",
		"BASE_URL=https://example.com",
	}, "\n"))
	env := map[string]string{"CONFIG_FILE": path, "GITHUB_CLIENT_ID": "env-cid"}
	lookup := func(k string) (string, bool) { v, ok := env[k]; return v, ok }
	c, err := readConfig(lookup)
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "file-db" || c.GitHubClientID != "env-cid" {
		t.Fatalf("config = %+v, want file DB and env client id", c)
	}
}

func TestReadConfigExplicitMissingFileFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.env")
	_, err := readConfig(func(k string) (string, bool) { return missing, k == "CONFIG_FILE" })
	if err == nil || !strings.Contains(err.Error(), "CONFIG_FILE") {
		t.Fatalf("err = %v, want CONFIG_FILE error", err)
	}
}

func TestReadConfigDefaultFileMissingIsFine(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := readConfig(func(string) (string, bool) { return "", false })
	if err == nil || !strings.Contains(err.Error(), "missing required env") {
		t.Fatalf("err = %v, want missing required env", err)
	}
}
