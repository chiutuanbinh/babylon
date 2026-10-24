package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"babylon/internal/adapters/github"
	"babylon/internal/adapters/postgres"
	"babylon/internal/adapters/sim"
	"babylon/internal/adapters/system"
	"babylon/internal/adapters/web"
	"babylon/internal/identity"
)

//go:embed templates
var assets embed.FS

var tmpl = template.Must(template.ParseFS(assets, "templates/*.html"))

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Bound boot-time DB work so an unreachable database cannot hang startup.
	bootCtx, cancelBoot := context.WithTimeout(ctx, 10*time.Second)
	pool, err := pgxpool.New(bootCtx, cfg.DatabaseURL)
	if err != nil {
		cancelBoot()
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := postgres.Migrate(bootCtx, pool); err != nil {
		cancelBoot()
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	cancelBoot()

	auth := &identity.Service{
		Users:    postgres.NewUserRepo(pool),
		Sessions: postgres.NewSessionRepo(pool),
		Provider: github.New(github.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.BaseURL + "/auth/callback",
		}),
		Tokens:     system.Tokens{},
		Clock:      system.Clock{},
		Admins:     cfg.AdminGitHubIDs,
		SessionTTL: 7 * 24 * time.Hour,
	}
	ui := &web.Handler{
		Auth:          auth,
		Tokens:        system.Tokens{},
		Log:           log,
		SecureCookies: cfg.secureCookies(),
	}

	simUI, err := sim.New()
	if err != nil {
		log.Error("sim assets", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           buildHandler(log, ui.Mount, simUI.Mount),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown failed", "err", err)
		os.Exit(1)
	}
}

// buildHandler is the full production chain: logging, security headers, routes.
func buildHandler(log *slog.Logger, mounts ...func(*http.ServeMux)) http.Handler {
	return logging(log, securityHeaders(routes(mounts...)))
}

// routes builds the mux. Mounts let adapters register their own routes; with
// none given it serves only the demo routes, which is what most tests want.
func routes(mounts ...func(*http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("POST /greet", handleGreet)
	mux.HandleFunc("GET /time", handleTime)
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /hello/{name}", handleHello)
	for _, mount := range mounts {
		mount(mux)
	}
	return mux
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	render(w, "index", struct{ Now time.Time }{time.Now()})
}

func handleGreet(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	render(w, "greeting", name)
}

func handleTime(w http.ResponseWriter, r *http.Request) {
	render(w, "time", time.Now())
}

// render executes a named template into a buffer first so a template error
// yields a clean 500 instead of a half-written page.
func render(w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "hello, " + r.PathValue("name")})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real connection (deadlines, flush).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func logging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur", time.Since(start),
		)
	})
}

// securityHeaders sets baseline hardening headers on every response. Scripts
// are same-origin only (htmx and three.js are vendored); the CSP also allows
// inline <style> for the demo page and the simulator pages.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
