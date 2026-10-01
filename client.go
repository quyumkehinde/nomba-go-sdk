// Package nomba is a Go client for the Nomba API.
package nomba

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Environment selects which Nomba API host the client talks to.
type Environment string

const (
	// Sandbox is Nomba's test environment. It is the default.
	Sandbox Environment = "sandbox"
	// Live is Nomba's production environment.
	Live Environment = "live"
)

var environmentURLs = map[Environment]string{
	Sandbox: "https://sandbox.nomba.com",
	Live:    "https://api.nomba.com",
}

const defaultTimeout = 30 * time.Second

// Client sends requests to the Nomba API. Create one with NewClient.
type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	accountID   string
	accessToken string
}

// Option configures a Client.
type Option func(*config) error

type config struct {
	env         Environment
	baseURL     string
	httpClient  *http.Client
	accountID   string
	accessToken string
}

// WithEnvironment selects Sandbox or Live. It is ignored when WithBaseURL is set.
func WithEnvironment(env Environment) Option {
	return func(c *config) error {
		if _, ok := environmentURLs[env]; !ok {
			return fmt.Errorf("nomba: unknown environment %q", env)
		}
		c.env = env
		return nil
	}
}

// WithBaseURL overrides the API host, for example to point at a test server.
// It takes precedence over WithEnvironment.
func WithBaseURL(rawURL string) Option {
	return func(c *config) error {
		if rawURL == "" {
			return errors.New("nomba: base URL is empty")
		}
		c.baseURL = rawURL
		return nil
	}
}

// WithHTTPClient sets the http.Client used to send requests.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *config) error {
		if hc == nil {
			return errors.New("nomba: http client is nil")
		}
		c.httpClient = hc
		return nil
	}
}

// WithAccountID sets the accountId header sent on requests that need it.
func WithAccountID(id string) Option {
	return func(c *config) error {
		c.accountID = id
		return nil
	}
}

// WithAccessToken sets a static access token sent as a Bearer token.
func WithAccessToken(token string) Option {
	return func(c *config) error {
		c.accessToken = token
		return nil
	}
}

// NewClient returns a Client configured by opts. It defaults to the Sandbox
// environment and an http.Client with a 30 second timeout.
func NewClient(opts ...Option) (*Client, error) {
	cfg := config{env: Sandbox}
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	rawURL := cfg.baseURL
	if rawURL == "" {
		rawURL = environmentURLs[cfg.env]
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("nomba: invalid base URL: %w", err)
	}
	if !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("nomba: base URL %q must be absolute", rawURL)
	}

	hc := cfg.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:     u,
		httpClient:  hc,
		accountID:   cfg.accountID,
		accessToken: cfg.accessToken,
	}, nil
}
