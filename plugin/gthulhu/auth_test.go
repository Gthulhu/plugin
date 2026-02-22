package gthulhu

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
	"strings"
	"testing"
	"time"

	reg "github.com/Gthulhu/plugin/plugin/internal/registry"
)

// authTestCerts holds PEM-encoded self-signed CA + leaf cert for unit testing.
type authTestCerts struct {
	caPEM   string
	certPEM string
	keyPEM  string
}

// generateAuthTestCerts creates a minimal self-signed CA and a leaf cert/key signed by it.
func generateAuthTestCerts(t *testing.T) authTestCerts {
	t.Helper()

	notBefore := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	// Generate CA key + cert
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))

	// Generate leaf key + cert signed by CA
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-leaf"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}))

	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}))

	return authTestCerts{caPEM: caPEM, certPEM: certPEM, keyPEM: keyPEM}
}

func TestNewJWTClientMTLSDisabled(t *testing.T) {
	mtlsCfg := reg.MTLSConfig{Enable: false}
	c, err := NewJWTClient("", "http://localhost", false, mtlsCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil JWTClient")
	}
	// Transport should be nil (plain http.Client default)
	if c.httpClient.Transport != nil {
		t.Errorf("expected nil transport for plain HTTP, got %T", c.httpClient.Transport)
	}
}

func TestNewJWTClientMTLSBadCert(t *testing.T) {
	mtlsCfg := reg.MTLSConfig{
		Enable:  true,
		CertPem: "not-valid-pem",
		KeyPem:  "not-valid-pem",
		CAPem:   "not-valid-pem",
	}
	_, err := NewJWTClient("", "https://localhost", false, mtlsCfg)
	if err == nil {
		t.Fatal("expected error for invalid cert PEM, got nil")
	}
	if !strings.Contains(err.Error(), "load mTLS client certificate") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewJWTClientMTLSBadCA(t *testing.T) {
	certs := generateAuthTestCerts(t)
	mtlsCfg := reg.MTLSConfig{
		Enable:  true,
		CertPem: certs.certPEM,
		KeyPem:  certs.keyPEM,
		CAPem:   "not-a-valid-ca-pem",
	}
	_, err := NewJWTClient("", "https://localhost", false, mtlsCfg)
	if err == nil {
		t.Fatal("expected error for invalid CA PEM, got nil")
	}
	if !strings.Contains(err.Error(), "parse mTLS CA certificate") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewJWTClientMTLSEnabled(t *testing.T) {
	certs := generateAuthTestCerts(t)
	mtlsCfg := reg.MTLSConfig{
		Enable:  true,
		CertPem: certs.certPEM,
		KeyPem:  certs.keyPEM,
		CAPem:   certs.caPEM,
	}
	c, err := NewJWTClient("", "https://localhost", false, mtlsCfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil JWTClient")
	}
	// Transport should be an mTLS-capable *http.Transport
	if _, ok := c.httpClient.Transport.(*http.Transport); !ok {
		t.Errorf("expected *http.Transport, got %T", c.httpClient.Transport)
	}
}

// TestNewJWTClientMTLSEndToEnd verifies that a JWTClient configured with mTLS
// can successfully complete a round-trip against an mTLS-enforcing httptest server.
func TestNewJWTClientMTLSEndToEnd(t *testing.T) {
	certs := generateAuthTestCerts(t)

	// Build mTLS test server that requires a client cert signed by our CA.
	serverCert, err := tls.X509KeyPair([]byte(certs.certPEM), []byte(certs.keyPEM))
	if err != nil {
		t.Fatalf("load server cert: %v", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM([]byte(certs.caPEM)) {
		t.Fatal("append CA cert")
	}
	serverTLSCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.TLS = serverTLSCfg
	server.StartTLS()
	defer server.Close()

	mtlsCfg := reg.MTLSConfig{
		Enable:  true,
		CertPem: certs.certPEM,
		KeyPem:  certs.keyPEM,
		CAPem:   certs.caPEM,
	}
	// authEnabled=false so no token fetch is attempted; we only verify the TLS handshake.
	c, err := NewJWTClient("", server.URL, false, mtlsCfg)
	if err != nil {
		t.Fatalf("NewJWTClient: %v", err)
	}

	resp, err := c.MakeAuthenticatedRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("MakeAuthenticatedRequest over mTLS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want %d", resp.StatusCode, http.StatusOK)
	}
}

