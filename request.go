package nomba

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	// maxSnippet caps how much of a response body is kept on an error.
	maxSnippet = 1 << 10
	// maxResponseBody caps how much of a response body is read.
	maxResponseBody = 10 << 20
)

// successCodes are the envelope codes Nomba uses for a successful call.
var successCodes = map[string]bool{"00": true, "200": true, "201": true}

// envelope is the wrapper Nomba puts around every response body.
type envelope struct {
	Code        *string         `json:"code"`
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
}

type requestConfig struct {
	skipAccountID bool
}

type requestOption func(*requestConfig)

// withoutAccountID stops the accountId header being sent, for the endpoints
// that don't take one.
func withoutAccountID() requestOption {
	return func(rc *requestConfig) { rc.skipAccountID = true }
}

// do sends a request to path (relative to the base URL, optionally with a
// query string), JSON-encoding body when it is non-nil. On success it decodes
// the envelope's data into out when out is non-nil and data is present. It
// returns *APIError for failure responses and *DecodeError for successful
// responses whose body can't be decoded.
func (c *Client) do(ctx context.Context, method, path string, body, out any, opts ...requestOption) error {
	var rc requestConfig
	for _, opt := range opts {
		opt(&rc)
	}

	u, err := c.resolve(path)
	if err != nil {
		return err
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("nomba: encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return fmt.Errorf("nomba: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
	if c.accountID != "" && !rc.skipAccountID {
		req.Header.Set("accountId", c.accountID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("nomba: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("nomba: reading response to %s %s: %w", method, path, err)
	}

	return decodeResponse(resp.StatusCode, raw, out)
}

// resolve joins path onto the base URL, keeping any path prefix the base URL has.
func (c *Client) resolve(path string) (string, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("nomba: invalid request path %q: %w", path, err)
	}
	u := *c.baseURL
	// Join the escaped forms so escaped characters such as %2F in an
	// identifier aren't decoded into path separators.
	rawPath := strings.TrimRight(u.EscapedPath(), "/") + "/" + strings.TrimLeft(ref.EscapedPath(), "/")
	if u.Path, err = url.PathUnescape(rawPath); err != nil {
		return "", fmt.Errorf("nomba: invalid request path %q: %w", path, err)
	}
	u.RawPath = rawPath
	u.RawQuery = ref.RawQuery
	return u.String(), nil
}

func decodeResponse(status int, raw []byte, out any) error {
	var env envelope
	parseErr := json.Unmarshal(raw, &env)
	if parseErr == nil && env.Code == nil {
		parseErr = errors.New(`response has no "code" field`)
	}

	if status < 200 || status > 299 {
		apiErr := &APIError{StatusCode: status, Kind: kindForStatus(status), RawBody: snippet(raw)}
		if parseErr == nil {
			apiErr.Code = *env.Code
			apiErr.Description = env.Description
		}
		return apiErr
	}

	if parseErr != nil {
		return &DecodeError{StatusCode: status, Body: snippet(raw), Err: parseErr}
	}
	if !successCodes[*env.Code] {
		return &APIError{
			StatusCode:  status,
			Code:        *env.Code,
			Description: env.Description,
			Kind:        KindUnknown,
			RawBody:     snippet(raw),
		}
	}

	if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return &DecodeError{StatusCode: status, Body: snippet(raw), Err: err}
	}
	return nil
}

func snippet(b []byte) string {
	if len(b) > maxSnippet {
		b = b[:maxSnippet]
	}
	return string(b)
}
