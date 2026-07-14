package auth

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

type JWT struct {
	tokenFile string
}

func NewJWT(tokenFile string) (*JWT, error) {
	if strings.TrimSpace(tokenFile) == "" {
		return nil, fmt.Errorf("OpenSVC daemon JWT file path is empty")
	}
	return &JWT{tokenFile: tokenFile}, nil
}

func (a *JWT) Apply(request *http.Request) error {
	tokenBytes, err := os.ReadFile(a.tokenFile)
	if err != nil {
		return fmt.Errorf("read OpenSVC daemon JWT file %q: %w", a.tokenFile, err)
	}

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return fmt.Errorf("OpenSVC daemon JWT file %q is empty", a.tokenFile)
	}

	request.Header.Set("Authorization", "Bearer "+token)
	return nil
}
