package auth

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
)

type jwtClaims struct {
	Grant    []string `json:"grant"`
	TokenUse string   `json:"token_use"`
	jwt.RegisteredClaims
}

// JWTVerifier validates OpenSVC access JWTs signed by the cluster CA.
type JWTVerifier struct {
	publicKey *rsa.PublicKey
}

// NewJWTVerifier loads the RSA public key from an OpenSVC cluster CA certificate or public-key file.
func NewJWTVerifier(verifyKeyFile string) (*JWTVerifier, error) {
	if strings.TrimSpace(verifyKeyFile) == "" {
		return nil, fmt.Errorf("OpenSVC JWT verification key file path is empty")
	}
	keyPEM, err := os.ReadFile(verifyKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read OpenSVC JWT verification key file %q: %w", verifyKeyFile, err)
	}
	publicKey, err := jwt.ParseRSAPublicKeyFromPEM(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse OpenSVC JWT RSA verification key file %q: %w", verifyKeyFile, err)
	}
	return &JWTVerifier{publicKey: publicKey}, nil
}

// Verify implements the MCP SDK bearer-token verifier contract.
func (v *JWTVerifier) Verify(_ context.Context, rawToken string, _ *http.Request) (*mcpauth.TokenInfo, error) {
	claims := &jwtClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodRS256 {
				return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
			}
			return v.publicKey, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return nil, invalidToken("signature or registered claims validation failed")
	}
	if claims.Subject == "" {
		return nil, invalidToken("subject claim is missing")
	}
	if claims.Issuer == "" {
		return nil, invalidToken("issuer claim is missing")
	}
	if claims.TokenUse != "access" {
		return nil, invalidToken("token_use claim is not access")
	}
	if claims.ExpiresAt == nil {
		return nil, invalidToken("expiration claim is missing")
	}

	return &mcpauth.TokenInfo{
		UserID:     claims.Subject,
		Scopes:     append([]string(nil), claims.Grant...),
		Expiration: claims.ExpiresAt.Time,
		Extra: map[string]any{
			"issuer":    claims.Issuer,
			"token_use": claims.TokenUse,
		},
	}, nil
}

func invalidToken(reason string) error {
	return errors.Join(mcpauth.ErrInvalidToken, errors.New(reason))
}
