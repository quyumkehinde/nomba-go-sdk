package nomba

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestNewClientDefaultsToSandbox(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got, want := c.baseURL.String(), "https://sandbox.nomba.com"; got != want {
		t.Errorf("baseURL = %q, want %q", got, want)
	}
}

func TestNewClientLiveEnvironment(t *testing.T) {
	c, err := NewClient(WithEnvironment(Live))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got, want := c.baseURL.String(), "https://api.nomba.com"; got != want {
		t.Errorf("baseURL = %q, want %q", got, want)
	}
}

func TestNewClientBaseURLOverridesEnvironment(t *testing.T) {
	for _, opts := range [][]Option{
		{WithBaseURL("http://localhost:8080"), WithEnvironment(Live)},
		{WithEnvironment(Live), WithBaseURL("http://localhost:8080")},
	} {
		c, err := NewClient(opts...)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		if got, want := c.baseURL.String(), "http://localhost:8080"; got != want {
			t.Errorf("baseURL = %q, want %q", got, want)
		}
	}
}

func TestNewClientRejectsInvalidOptions(t *testing.T) {
	tests := map[string]Option{
		"unparseable base URL": WithBaseURL("://bad"),
		"relative base URL":    WithBaseURL("/v1"),
		"empty base URL":       WithBaseURL(""),
		"nil http client":      WithHTTPClient(nil),
		"unknown environment":  WithEnvironment(Environment("staging")),
	}
	for name, opt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewClient(opt); err == nil {
				t.Fatal("NewClient returned nil error")
			}
		})
	}
}

func TestNewClientDefaultHTTPClientHasTimeout(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.httpClient == nil || c.httpClient.Timeout == 0 {
		t.Fatal("default http client has no timeout")
	}
	if c.httpClient == http.DefaultClient {
		t.Fatal("default http client must not be http.DefaultClient")
	}
}

func TestNewClientUsesProvidedHTTPClient(t *testing.T) {
	hc := &http.Client{}
	c, err := NewClient(WithHTTPClient(hc))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.httpClient != hc {
		t.Fatal("provided http client not used")
	}
}

func TestNewClientStoresCredentials(t *testing.T) {
	c, err := NewClient(WithAccountID("acc"), WithAccessToken("tok"))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.accountID != "acc" || c.accessToken != "tok" {
		t.Errorf("accountID=%q accessToken=%q", c.accountID, c.accessToken)
	}
}

func TestGoModHasNoDependencies(t *testing.T) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "require") {
		t.Fatalf("go.mod must not require modules (stdlib only):\n%s", b)
	}
}
