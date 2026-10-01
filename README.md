# Nomba Go SDK

A Go client for the [Nomba API](https://developer.nomba.com). Standard library only.

> Early stage: the client foundation is in place; product endpoints are coming.

## Install

```sh
go get github.com/quyumkehinde/nomba-go-sdk
```

## Usage

```go
client, err := nomba.NewClient(
	nomba.WithEnvironment(nomba.Live), // defaults to nomba.Sandbox
	nomba.WithAccountID("your-account-id"),
	nomba.WithAccessToken("your-access-token"),
)
if err != nil {
	log.Fatal(err)
}
```

Other options:

- `WithBaseURL(url)` overrides the API host (takes precedence over `WithEnvironment`).
- `WithHTTPClient(hc)` uses your own `*http.Client` (default has a 30s timeout).

## Errors

Failed API calls return a `*nomba.APIError`. Check the kind with `errors.Is`:

```go
if errors.Is(err, nomba.ErrNotFound) {
	// ...
}

var apiErr *nomba.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.StatusCode, apiErr.Code, apiErr.Description)
}
```

Sentinels: `ErrBadRequest`, `ErrUnauthorized`, `ErrForbidden`, `ErrNotFound`, `ErrRateLimited`, `ErrServer`.

A successful response with a body that can't be decoded returns a `*nomba.DecodeError`.

## Development

```sh
go test -race ./...
```
