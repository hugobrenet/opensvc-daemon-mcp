package auth

import "net/http"

// Authenticator applies daemon authentication to an HTTP request.
type Authenticator interface {
	Apply(*http.Request) error
}

// None leaves requests unauthenticated. It is intended for tests and fake daemons.
type None struct{}

func (None) Apply(_ *http.Request) error {
	return nil
}
