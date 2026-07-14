package auth

import (
	"fmt"
	"net/http"
)

// Options describes the daemon request authentication configuration.
type Options struct {
	Method            string
	TokenFile         string
	BasicUsername     string
	BasicPasswordFile string
}

// Authenticator applies daemon authentication to an HTTP request.
type Authenticator interface {
	Apply(*http.Request) error
}

// None leaves requests unauthenticated. It is intended for tests and fake daemons.
type None struct{}

func (None) Apply(_ *http.Request) error {
	return nil
}

// X509 leaves HTTP headers unchanged because authentication happens during the TLS handshake.
type X509 struct{}

func (X509) Apply(_ *http.Request) error {
	return nil
}

// New selects and constructs the configured request authenticator.
func New(options Options) (Authenticator, error) {
	switch options.Method {
	case "jwt":
		return NewJWT(options.TokenFile)
	case "basic":
		return NewBasic(options.BasicUsername, options.BasicPasswordFile)
	case "x509":
		return X509{}, nil
	case "none":
		return None{}, nil
	default:
		return nil, fmt.Errorf("unsupported OpenSVC daemon authentication method %q", options.Method)
	}
}
