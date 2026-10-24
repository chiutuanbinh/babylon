// Package github implements identity.IdentityProvider with GitHub OAuth.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	oauth2gh "golang.org/x/oauth2/github"

	"babylon/internal/identity"
)

// Config holds the OAuth app credentials. AuthURL, TokenURL and APIURL default
// to GitHub's; tests point them at a fake server.
type Config struct {
	ClientID, ClientSecret, RedirectURL string
	AuthURL, TokenURL, APIURL           string
}

// exchangeTimeout caps a whole Exchange; a variable only so tests can shrink it.
var exchangeTimeout = 8 * time.Second

var _ identity.IdentityProvider = (*Provider)(nil)

// Provider is the GitHub OAuth implementation of identity.IdentityProvider.
type Provider struct {
	oauth  *oauth2.Config
	apiURL string
	client *http.Client
}

// New builds a Provider from c, defaulting any empty URL to GitHub's.
func New(c Config) *Provider {
	ep := oauth2gh.Endpoint
	ep.AuthStyle = oauth2.AuthStyleInParams // deterministic: no retry in the other style
	if c.AuthURL != "" {
		ep.AuthURL = c.AuthURL
	}
	if c.TokenURL != "" {
		ep.TokenURL = c.TokenURL
	}
	api := c.APIURL
	if api == "" {
		api = "https://api.github.com"
	}
	return &Provider{
		oauth: &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Endpoint:     ep,
		},
		apiURL: api,
		// Per-request cap; Exchange also bounds the whole flow with a deadline.
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *Provider) AuthURL(state string) string {
	return p.oauth.AuthCodeURL(state)
}

// Exchange trades the OAuth code for a token and fetches the GitHub user. The
// whole exchange (token request plus /user call) is capped at 8s, below the
// server's 10s WriteTimeout, so a slow GitHub yields our error, not a dropped
// connection.
func (p *Provider) Exchange(ctx context.Context, code string) (identity.Profile, error) {
	ctx, cancel := context.WithTimeout(ctx, exchangeTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	tok, err := p.oauth.Exchange(ctx, code)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: exchange code: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiURL+"/user", nil)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := p.oauth.Client(ctx, tok).Do(req)
	if err != nil {
		return identity.Profile{}, fmt.Errorf("github: get user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return identity.Profile{}, fmt.Errorf("github: get user: status %d", resp.StatusCode)
	}

	var u struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&u); err != nil {
		return identity.Profile{}, fmt.Errorf("github: decode user: %w", err)
	}
	if u.ID == 0 || u.Login == "" {
		return identity.Profile{}, fmt.Errorf("github: user response missing id or login")
	}
	return identity.Profile{GitHubID: u.ID, Login: u.Login}, nil
}
