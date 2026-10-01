package nomba

import (
	"errors"
	"fmt"
	"net/http"
)

// Kind classifies an APIError by the HTTP status of the response.
type Kind string

const (
	KindBadRequest   Kind = "bad_request"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindRateLimited  Kind = "rate_limited"
	KindServer       Kind = "server_error"
	// KindUnknown covers any other failure, such as a 409 or a 2xx response
	// whose envelope code is not a success code.
	KindUnknown Kind = "unknown"
)

// Sentinel errors for matching an APIError's Kind with errors.Is.
var (
	ErrBadRequest   = errors.New("nomba: bad request")
	ErrUnauthorized = errors.New("nomba: unauthorized")
	ErrForbidden    = errors.New("nomba: forbidden")
	ErrNotFound     = errors.New("nomba: not found")
	ErrRateLimited  = errors.New("nomba: rate limited")
	ErrServer       = errors.New("nomba: server error")
)

var kindSentinels = map[Kind]error{
	KindBadRequest:   ErrBadRequest,
	KindUnauthorized: ErrUnauthorized,
	KindForbidden:    ErrForbidden,
	KindNotFound:     ErrNotFound,
	KindRateLimited:  ErrRateLimited,
	KindServer:       ErrServer,
}

// APIError is returned when Nomba responds with a failure. Code and
// Description come from the {code, description} response body and are empty
// when the body could not be parsed; RawBody then holds the start of it.
type APIError struct {
	StatusCode  int
	Code        string
	Description string
	Kind        Kind
	RawBody     string
}

func (e *APIError) Error() string {
	if e.Code == "" && e.Description == "" {
		return fmt.Sprintf("nomba: HTTP %d (%s)", e.StatusCode, e.Kind)
	}
	return fmt.Sprintf("nomba: HTTP %d (%s): code %q: %s", e.StatusCode, e.Kind, e.Code, e.Description)
}

// Is reports whether target is the sentinel error for e's Kind.
func (e *APIError) Is(target error) bool {
	s, ok := kindSentinels[e.Kind]
	return ok && s == target
}

// DecodeError is returned when a successful HTTP response has a body that is
// not a valid Nomba response.
type DecodeError struct {
	StatusCode int
	Body       string
	Err        error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("nomba: decoding HTTP %d response: %v", e.StatusCode, e.Err)
}

func (e *DecodeError) Unwrap() error { return e.Err }

func kindForStatus(status int) Kind {
	switch {
	case status == http.StatusBadRequest:
		return KindBadRequest
	case status == http.StatusUnauthorized:
		return KindUnauthorized
	case status == http.StatusForbidden:
		return KindForbidden
	case status == http.StatusNotFound:
		return KindNotFound
	case status == http.StatusTooManyRequests:
		return KindRateLimited
	case status >= 500 && status <= 599:
		return KindServer
	default:
		return KindUnknown
	}
}
