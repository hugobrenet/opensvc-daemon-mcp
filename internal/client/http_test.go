package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHTTPClientTLSVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	verifiedClient, err := NewHTTPClient(HTTPOptions{})
	if err != nil {
		t.Fatalf("create verified HTTP client: %v", err)
	}
	verifiedResponse, err := verifiedClient.Get(server.URL)
	if err == nil {
		verifiedResponse.Body.Close()
		t.Fatal("verified TLS request succeeded with an untrusted certificate")
	}

	insecureClient, err := NewHTTPClient(HTTPOptions{TLSInsecure: true})
	if err != nil {
		t.Fatalf("create explicitly insecure HTTP client: %v", err)
	}
	insecureResponse, err := insecureClient.Get(server.URL)
	if err != nil {
		t.Fatalf("explicitly insecure TLS request failed: %v", err)
	}
	defer insecureResponse.Body.Close()
	if insecureResponse.StatusCode != http.StatusNoContent {
		t.Errorf("got HTTP status %s, want 204 No Content", insecureResponse.Status)
	}
}

func TestNewHTTPClientRejectsIncompleteClientCertificatePair(t *testing.T) {
	_, err := NewHTTPClient(HTTPOptions{TLSClientCertificateFile: "/tmp/client.crt"})
	if err == nil {
		t.Fatal("NewHTTPClient succeeded, want an error")
	}
}

func TestNewHTTPClientRejectsMissingTLSFiles(t *testing.T) {
	tests := []struct {
		name    string
		options HTTPOptions
	}{
		{
			name:    "CA",
			options: HTTPOptions{TLSCAFile: "/missing/ca.crt"},
		},
		{
			name: "client certificate",
			options: HTTPOptions{
				TLSClientCertificateFile: "/missing/client.crt",
				TLSClientKeyFile:         "/missing/client.key",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewHTTPClient(test.options); err == nil {
				t.Fatal("NewHTTPClient succeeded, want an error")
			}
		})
	}
}

func TestHTTPClientMutualTLS(t *testing.T) {
	caCertificate, caKey, caPEM, _ := issueTestCertificate(t, nil, nil, certificateTemplate{
		commonName: "test CA",
		isCA:       true,
	})
	_, _, serverPEM, serverKeyPEM := issueTestCertificate(t, caCertificate, caKey, certificateTemplate{
		commonName: "test server",
		ipAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
		},
		extKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	_, _, clientPEM, clientKeyPEM := issueTestCertificate(t, caCertificate, caKey, certificateTemplate{
		commonName:  "opensvc-daemon-mcp",
		extKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	serverCertificate, err := tls.X509KeyPair(serverPEM, serverKeyPEM)
	if err != nil {
		t.Fatalf("load test server certificate: %v", err)
	}
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(caCertificate)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.TLS == nil || len(request.TLS.PeerCertificates) == 0 {
			t.Error("request has no verified client certificate")
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()
	defer server.Close()

	directory := t.TempDir()
	caFile := writeTLSFile(t, directory, "ca.crt", caPEM)
	clientCertificateFile := writeTLSFile(t, directory, "client.crt", clientPEM)
	clientKeyFile := writeTLSFile(t, directory, "client.key", clientKeyPEM)

	httpClient, err := NewHTTPClient(HTTPOptions{
		TLSCAFile:                caFile,
		TLSClientCertificateFile: clientCertificateFile,
		TLSClientKeyFile:         clientKeyFile,
	})
	if err != nil {
		t.Fatalf("create mutual TLS client: %v", err)
	}
	response, err := httpClient.Get(server.URL)
	if err != nil {
		t.Fatalf("mutual TLS request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Errorf("got HTTP status %s, want 204 No Content", response.Status)
	}
}

type certificateTemplate struct {
	commonName  string
	isCA        bool
	ipAddresses []net.IP
	extKeyUsage []x509.ExtKeyUsage
}

func issueTestCertificate(
	t *testing.T,
	caCertificate *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	options certificateTemplate,
) (*x509.Certificate, *ecdsa.PrivateKey, []byte, []byte) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: options.commonName},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  options.isCA,
		IPAddresses:           options.ipAddresses,
		ExtKeyUsage:           options.extKeyUsage,
		KeyUsage:              x509.KeyUsageDigitalSignature,
	}
	if options.isCA {
		template.KeyUsage |= x509.KeyUsageCertSign
		caCertificate = template
		caKey = privateKey
	}

	certificateDER, err := x509.CreateCertificate(rand.Reader, template, caCertificate, &privateKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create test certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(certificateDER)
	if err != nil {
		t.Fatalf("parse test certificate: %v", err)
	}
	privateKeyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal test private key: %v", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDER})
	return certificate, privateKey, certificatePEM, privateKeyPEM
}

func writeTLSFile(t *testing.T, directory string, name string, content []byte) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write TLS file %s: %v", name, err)
	}
	return path
}
