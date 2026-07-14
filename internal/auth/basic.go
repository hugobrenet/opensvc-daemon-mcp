package auth

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

type Basic struct {
	username     string
	passwordFile string
}

func NewBasic(username string, passwordFile string) (*Basic, error) {
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("OpenSVC daemon Basic Auth username is empty")
	}
	if strings.TrimSpace(passwordFile) == "" {
		return nil, fmt.Errorf("OpenSVC daemon Basic Auth password file path is empty")
	}
	return &Basic{username: username, passwordFile: passwordFile}, nil
}

func (a *Basic) Apply(request *http.Request) error {
	passwordBytes, err := os.ReadFile(a.passwordFile)
	if err != nil {
		return fmt.Errorf("read OpenSVC daemon Basic Auth password file %q: %w", a.passwordFile, err)
	}

	password := string(passwordBytes)
	if strings.HasSuffix(password, "\r\n") {
		password = strings.TrimSuffix(password, "\r\n")
	} else {
		password = strings.TrimSuffix(password, "\n")
	}
	if password == "" {
		return fmt.Errorf("OpenSVC daemon Basic Auth password file %q is empty", a.passwordFile)
	}

	request.SetBasicAuth(a.username, password)
	return nil
}
