package nomba

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

var allSentinels = []error{ErrBadRequest, ErrUnauthorized, ErrForbidden, ErrNotFound, ErrRateLimited, ErrServer}

func TestKindForStatus(t *testing.T) {
	tests := []struct {
		status int
		want   Kind
	}{
		{400, KindBadRequest},
		{401, KindUnauthorized},
		{403, KindForbidden},
		{404, KindNotFound},
		{429, KindRateLimited},
		{500, KindServer},
		{502, KindServer},
		{503, KindServer},
		{599, KindServer},
		{200, KindUnknown},
		{302, KindUnknown},
		{409, KindUnknown},
		{422, KindUnknown},
	}
	for _, tt := range tests {
		if got := kindForStatus(tt.status); got != tt.want {
			t.Errorf("kindForStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestAPIErrorMatchesOnlyItsSentinel(t *testing.T) {
	tests := []struct {
		kind Kind
		want error
	}{
		{KindBadRequest, ErrBadRequest},
		{KindUnauthorized, ErrUnauthorized},
		{KindForbidden, ErrForbidden},
		{KindNotFound, ErrNotFound},
		{KindRateLimited, ErrRateLimited},
		{KindServer, ErrServer},
		{KindUnknown, nil},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind), func(t *testing.T) {
			var err error = &APIError{Kind: tt.kind}
			for _, s := range allSentinels {
				if got, want := errors.Is(err, s), s == tt.want; got != want {
					t.Errorf("errors.Is(%v) = %v, want %v", s, got, want)
				}
			}
		})
	}
}

func TestAPIErrorMessage(t *testing.T) {
	err := &APIError{StatusCode: 404, Code: "404", Description: "Record not found", Kind: KindNotFound}
	msg := err.Error()
	for _, want := range []string{"404", "Record not found"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}
}

func TestDecodeErrorUnwraps(t *testing.T) {
	cause := &json.SyntaxError{}
	err := &DecodeError{StatusCode: 200, Body: "<html>", Err: cause}
	if !errors.Is(err, cause) {
		t.Fatal("DecodeError does not unwrap its cause")
	}
	if !strings.Contains(err.Error(), "200") {
		t.Errorf("Error() = %q, missing status", err.Error())
	}
}

func TestAPIErrorMessageWithoutBody(t *testing.T) {
	err := &APIError{StatusCode: 502, Kind: KindServer, RawBody: "<html>"}
	if msg := err.Error(); !strings.Contains(msg, "502") || !strings.Contains(msg, string(KindServer)) {
		t.Errorf("Error() = %q", msg)
	}
}
