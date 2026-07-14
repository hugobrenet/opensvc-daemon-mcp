package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTVerifier(t *testing.T) {
	privateKey, certificateFile := writeJWTTestCertificate(t)
	verifier, err := NewJWTVerifier(certificateFile)
	if err != nil {
		t.Fatalf("create JWT verifier: %v", err)
	}
	token := signTestJWT(t, privateKey, jwt.MapClaims{
		"exp":       time.Now().Add(time.Hour).Unix(),
		"grant":     []string{"guest", "operator"},
		"iss":       "node-a",
		"sub":       "alice",
		"token_use": "access",
	})

	info, err := verifier.Verify(t.Context(), token, nil)
	if err != nil {
		t.Fatalf("verify JWT: %v", err)
	}
	if info.UserID != "alice" {
		t.Errorf("got user ID %q, want alice", info.UserID)
	}
	if len(info.Scopes) != 2 || info.Scopes[0] != "guest" || info.Scopes[1] != "operator" {
		t.Errorf("got scopes %#v, want OpenSVC grants", info.Scopes)
	}
}

func TestJWTVerifierRejectsInvalidClaims(t *testing.T) {
	privateKey, certificateFile := writeJWTTestCertificate(t)
	verifier, err := NewJWTVerifier(certificateFile)
	if err != nil {
		t.Fatalf("create JWT verifier: %v", err)
	}

	valid := jwt.MapClaims{
		"exp":       time.Now().Add(time.Hour).Unix(),
		"grant":     []string{"guest"},
		"iss":       "node-a",
		"sub":       "alice",
		"token_use": "access",
	}
	for _, test := range []struct {
		name   string
		change func(jwt.MapClaims)
	}{
		{name: "expired", change: func(claims jwt.MapClaims) { claims["exp"] = time.Now().Add(-time.Minute).Unix() }},
		{name: "missing subject", change: func(claims jwt.MapClaims) { delete(claims, "sub") }},
		{name: "missing issuer", change: func(claims jwt.MapClaims) { delete(claims, "iss") }},
		{name: "refresh token", change: func(claims jwt.MapClaims) { claims["token_use"] = "refresh" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := jwt.MapClaims{}
			for key, value := range valid {
				claims[key] = value
			}
			test.change(claims)
			token := signTestJWT(t, privateKey, claims)
			if _, err := verifier.Verify(t.Context(), token, nil); err == nil {
				t.Fatal("Verify succeeded, want an error")
			}
		})
	}
}

func TestJWTVerifierRejectsInvalidSignatureAndAlgorithm(t *testing.T) {
	_, certificateFile := writeJWTTestCertificate(t)
	verifier, err := NewJWTVerifier(certificateFile)
	if err != nil {
		t.Fatalf("create JWT verifier: %v", err)
	}
	claims := jwt.MapClaims{
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iss":       "node-a",
		"sub":       "alice",
		"token_use": "access",
	}

	untrustedKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate untrusted RSA key: %v", err)
	}
	wrongSignature := signTestJWT(t, untrustedKey, claims)
	if _, err := verifier.Verify(t.Context(), wrongSignature, nil); err == nil {
		t.Fatal("Verify accepted a token signed by an untrusted key")
	}

	wrongAlgorithm, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("shared-secret"))
	if err != nil {
		t.Fatalf("sign HMAC test JWT: %v", err)
	}
	if _, err := verifier.Verify(t.Context(), wrongAlgorithm, nil); err == nil {
		t.Fatal("Verify accepted a token using HS256")
	}
}

func TestNewJWTVerifierRejectsMissingFile(t *testing.T) {
	if _, err := NewJWTVerifier(filepath.Join(t.TempDir(), "missing.pem")); err == nil {
		t.Fatal("NewJWTVerifier succeeded, want an error")
	}
}

func writeJWTTestCertificate(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "OpenSVC test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}
	certificateFile := filepath.Join(t.TempDir(), "ca.pem")
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	if err := os.WriteFile(certificateFile, certificatePEM, 0o600); err != nil {
		t.Fatalf("write test certificate: %v", err)
	}
	return privateKey, certificateFile
}

func signTestJWT(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privateKey)
	if err != nil {
		t.Fatalf("sign test JWT: %v", err)
	}
	return token
}
