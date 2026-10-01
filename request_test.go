package nomba

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type testData struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

// newTestClient starts a server that runs handler and returns a client pointed at it.
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(append([]Option{WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		io.WriteString(w, body)
	}
}

func TestDoSendsAccountIDHeader(t *testing.T) {
	var got []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Values("accountId")
		io.WriteString(w, `{"code":"00","description":"Success","data":null}`)
	}, WithAccountID("acc"))

	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "acc" {
		t.Errorf("accountId = %v, want [acc]", got)
	}
}

func TestDoOmitsAccountIDWhenOptedOut(t *testing.T) {
	var present bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Accountid"]
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	}, WithAccountID("acc"))

	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil, withoutAccountID()); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Error("accountId header sent despite withoutAccountID")
	}
}

func TestDoOmitsAccountIDWhenUnset(t *testing.T) {
	var present bool
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Accountid"]
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	})

	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Error("accountId header sent without an account ID")
	}
}

func TestDoAuthorizationHeader(t *testing.T) {
	for _, tt := range []struct {
		name  string
		opts  []Option
		want  string
		wantN int
	}{
		{"with token", []Option{WithAccessToken("tok")}, "Bearer tok", 1},
		{"without token", nil, "", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Values("Authorization")
				io.WriteString(w, `{"code":"00","description":"Success"}`)
			}, tt.opts...)
			if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantN || (tt.wantN == 1 && got[0] != tt.want) {
				t.Errorf("Authorization = %v, want %q", got, tt.want)
			}
		})
	}
}

func TestDoEncodesJSONBody(t *testing.T) {
	var method, contentType, accept string
	var body testData
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		accept = r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	})

	in := testData{Name: "a", Amount: 5}
	if err := c.do(context.Background(), http.MethodPost, "/v1/x", in, nil); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost {
		t.Errorf("method = %q", method)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q", contentType)
	}
	if accept != "application/json" {
		t.Errorf("Accept = %q", accept)
	}
	if body != in {
		t.Errorf("body = %+v, want %+v", body, in)
	}
}

func TestDoWithoutBodySendsNoContentType(t *testing.T) {
	var contentType, accept string
	var n int64
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		accept = r.Header.Get("Accept")
		n, _ = io.Copy(io.Discard, r.Body)
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	})
	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if contentType != "" || n != 0 {
		t.Errorf("Content-Type = %q, body bytes = %d; want none", contentType, n)
	}
	if accept != "application/json" {
		t.Errorf("Accept = %q", accept)
	}
}

func TestDoJoinsPathToBaseURL(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	}))
	t.Cleanup(srv.Close)

	for _, tt := range []struct{ base, path, wantPath, wantQuery string }{
		{srv.URL, "/v1/accounts", "/v1/accounts", ""},
		{srv.URL, "v1/accounts", "/v1/accounts", ""},
		{srv.URL + "/prefix", "/v1/accounts", "/prefix/v1/accounts", ""},
		{srv.URL + "/prefix/", "/v1/accounts", "/prefix/v1/accounts", ""},
		{srv.URL, "/v1/accounts?limit=10&cursor=a", "/v1/accounts", "limit=10&cursor=a"},
	} {
		c, err := NewClient(WithBaseURL(tt.base))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.do(context.Background(), http.MethodGet, tt.path, nil, nil); err != nil {
			t.Fatal(err)
		}
		if gotPath != tt.wantPath || gotQuery != tt.wantQuery {
			t.Errorf("base %q + %q: got %q?%q, want %q?%q", tt.base, tt.path, gotPath, gotQuery, tt.wantPath, tt.wantQuery)
		}
	}
}

func TestDoPreservesEscapedPathSegments(t *testing.T) {
	var gotRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawPath = r.URL.EscapedPath()
		io.WriteString(w, `{"code":"00","description":"Success"}`)
	}))
	t.Cleanup(srv.Close)

	for _, tt := range []struct{ base, path, want string }{
		{srv.URL, "/v1/accounts/virtual/" + url.PathEscape("a/b"), "/v1/accounts/virtual/a%2Fb"},
		{srv.URL + "/pre%2Ffix", "/v1/x/" + url.PathEscape("a b"), "/pre%2Ffix/v1/x/a%20b"},
	} {
		c, err := NewClient(WithBaseURL(tt.base))
		if err != nil {
			t.Fatal(err)
		}
		if err := c.do(context.Background(), http.MethodGet, tt.path, nil, nil); err != nil {
			t.Fatal(err)
		}
		if gotRawPath != tt.want {
			t.Errorf("base %q + %q: sent path %q, want %q", tt.base, tt.path, gotRawPath, tt.want)
		}
	}
}

func TestDoRespectsContext(t *testing.T) {
	c := newTestClient(t, respond(200, `{"code":"00","description":"Success"}`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.do(ctx, http.MethodGet, "/v1/x", nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestDoUnencodableBodySendsNothing(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	err := c.do(context.Background(), http.MethodPost, "/v1/x", math.Inf(1), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if called {
		t.Error("request was sent")
	}
}

func TestDoUsesProvidedHTTPClient(t *testing.T) {
	used := false
	srv := httptest.NewServer(respond(200, `{"code":"00","description":"Success"}`))
	t.Cleanup(srv.Close)
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		used = true
		return http.DefaultTransport.RoundTrip(r)
	})}
	c, err := NewClient(WithBaseURL(srv.URL), WithHTTPClient(hc))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Error("custom http client was not used")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDoUnwrapsSuccessEnvelope(t *testing.T) {
	for _, tt := range []struct {
		status int
		code   string
	}{
		{200, "00"},
		{200, "200"},
		{201, "201"},
		{201, "00"},
	} {
		t.Run(tt.code, func(t *testing.T) {
			c := newTestClient(t, respond(tt.status, `{"code":"`+tt.code+`","description":"Success","data":{"name":"x","amount":7}}`))
			var out testData
			if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out); err != nil {
				t.Fatal(err)
			}
			if want := (testData{Name: "x", Amount: 7}); out != want {
				t.Errorf("out = %+v, want %+v", out, want)
			}
		})
	}
}

func TestDoSuccessWithNilTarget(t *testing.T) {
	c := newTestClient(t, respond(200, `{"code":"00","description":"Success","data":{"name":"x"}}`))
	if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDoSuccessWithoutDataLeavesTarget(t *testing.T) {
	for _, body := range []string{
		`{"code":"00","description":"Success"}`,
		`{"code":"00","description":"Success","data":null}`,
	} {
		c := newTestClient(t, respond(200, body))
		out := testData{Name: "keep"}
		if err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out); err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if out.Name != "keep" {
			t.Errorf("%s: target modified: %+v", body, out)
		}
	}
}

func TestDoTypedErrors(t *testing.T) {
	tests := []struct {
		status   int
		code     string
		sentinel error
		kind     Kind
	}{
		{400, "400", ErrBadRequest, KindBadRequest},
		{401, "401", ErrUnauthorized, KindUnauthorized},
		{403, "403", ErrForbidden, KindForbidden},
		{404, "404", ErrNotFound, KindNotFound},
		{429, "429", ErrRateLimited, KindRateLimited},
		{500, "500", ErrServer, KindServer},
		{502, "502", ErrServer, KindServer},
		{503, "503", ErrServer, KindServer},
		{409, "409", nil, KindUnknown},
		{200, "99", nil, KindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			c := newTestClient(t, respond(tt.status, `{"code":"`+tt.code+`","description":"something failed"}`))
			out := testData{Name: "keep"}
			err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out)

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v (%T), want *APIError", err, err)
			}
			if apiErr.StatusCode != tt.status || apiErr.Code != tt.code ||
				apiErr.Description != "something failed" || apiErr.Kind != tt.kind {
				t.Errorf("got %+v", apiErr)
			}
			for _, s := range allSentinels {
				if got, want := errors.Is(err, s), s == tt.sentinel; got != want {
					t.Errorf("errors.Is(%v) = %v, want %v", s, got, want)
				}
			}
			if out.Name != "keep" {
				t.Errorf("target modified on error: %+v", out)
			}
		})
	}
}

func TestDoErrorWithUnparseableBody(t *testing.T) {
	html := "<html><body>502 Bad Gateway</body></html>"
	for _, tt := range []struct {
		name     string
		status   int
		body     string
		sentinel error
	}{
		{"html 502", 502, html, ErrServer},
		{"empty 401", 401, "", ErrUnauthorized},
		{"empty 404", 404, "", ErrNotFound},
		{"json without code 400", 400, `{"error":"x"}`, ErrBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, respond(tt.status, tt.body))
			err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %v (%T), want *APIError", err, err)
			}
			if !errors.Is(err, tt.sentinel) {
				t.Errorf("errors.Is(%v) = false", tt.sentinel)
			}
			if apiErr.StatusCode != tt.status || apiErr.Code != "" {
				t.Errorf("got %+v", apiErr)
			}
			if apiErr.RawBody != tt.body {
				t.Errorf("RawBody = %q, want %q", apiErr.RawBody, tt.body)
			}
		})
	}
}

func TestDoDecodeErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{"html", "<html>ok</html>"},
		{"empty", ""},
		{"truncated json", `{"code":"00",`},
		{"missing code", `{"description":"Success","data":{}}`},
		{"code not a string", `{"code":0,"description":"Success"}`},
		{"json array", `[1,2]`},
		{"data type mismatch", `{"code":"00","description":"Success","data":{"amount":"lots"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, respond(200, tt.body))
			var out testData
			err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out)
			var decErr *DecodeError
			if !errors.As(err, &decErr) {
				t.Fatalf("err = %v (%T), want *DecodeError", err, err)
			}
			if decErr.StatusCode != 200 || decErr.Err == nil {
				t.Errorf("got %+v", decErr)
			}
			if !strings.Contains(err.Error(), "200") {
				t.Errorf("Error() = %q, missing status", err.Error())
			}
		})
	}
}

func TestDoDataMismatchWrapsJSONError(t *testing.T) {
	c := newTestClient(t, respond(200, `{"code":"00","description":"Success","data":{"amount":"lots"}}`))
	var out testData
	err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, &out)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("err = %v, want wrapped *json.UnmarshalTypeError", err)
	}
}

func TestDoCapsBodySnippet(t *testing.T) {
	big := strings.Repeat("x", 5000)

	c := newTestClient(t, respond(502, big))
	err := c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if len(apiErr.RawBody) != maxSnippet {
		t.Errorf("len(RawBody) = %d, want %d", len(apiErr.RawBody), maxSnippet)
	}

	c = newTestClient(t, respond(200, big))
	err = c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil)
	var decErr *DecodeError
	if !errors.As(err, &decErr) {
		t.Fatalf("err = %v, want *DecodeError", err)
	}
	if len(decErr.Body) != maxSnippet {
		t.Errorf("len(Body) = %d, want %d", len(decErr.Body), maxSnippet)
	}
}

func TestDoTransportError(t *testing.T) {
	srv := httptest.NewServer(respond(200, ""))
	c, err := NewClient(WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	srv.Close()

	err = c.do(context.Background(), http.MethodGet, "/v1/x", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	var decErr *DecodeError
	if errors.As(err, &apiErr) || errors.As(err, &decErr) {
		t.Errorf("transport failure surfaced as %T", err)
	}
	if errors.Unwrap(err) == nil {
		t.Errorf("err = %v does not wrap its cause", err)
	}
}

func TestDoTruncatedResponseBody(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		io.WriteString(w, `{"code":"00",`)
	})
	err := c.do(context.Background(), http.MethodPost, "/v1/x", nil, nil)
	if err == nil {
		t.Fatal("expected error for a body cut short")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("truncated body surfaced as *APIError: %v", err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("err = %v, want wrapped io.ErrUnexpectedEOF", err)
	}
}

func TestDoRejectsInvalidRequests(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	for name, call := range map[string]func() error{
		"invalid path":   func() error { return c.do(context.Background(), http.MethodGet, "/v1/%zz", nil, nil) },
		"invalid method": func() error { return c.do(context.Background(), "BAD METHOD", "/v1/x", nil, nil) },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if called {
		t.Error("request was sent")
	}
}
