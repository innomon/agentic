package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func generateTestKeys(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("failed to marshal public key: %v", err)
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	path := filepath.Join(t.TempDir(), "test_pub.pem")
	if err := os.WriteFile(path, pubPEM, 0644); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}

	return path, key
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenStr
}

func validClaims() Claims {
	now := time.Now()
	return Claims{
		UserID:  "919876543210",
		Channel: "whatsapp",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "whatsadk-gateway",
			Audience:  jwt.ClaimStrings{"adk-agent"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
}

func TestVerify_ValidToken(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	tokenStr := signToken(t, privKey, claims)

	got, err := verifier.Verify(tokenStr)
	if err != nil {
		t.Fatalf("Verify returned unexpected error: %v", err)
	}

	if got.UserID != "919876543210" {
		t.Errorf("UserID = %q, want %q", got.UserID, "919876543210")
	}
	if got.Channel != "whatsapp" {
		t.Errorf("Channel = %q, want %q", got.Channel, "whatsapp")
	}
	if got.Issuer != "whatsadk-gateway" {
		t.Errorf("Issuer = %q, want %q", got.Issuer, "whatsadk-gateway")
	}
	if len(got.Audience) == 0 || got.Audience[0] != "adk-agent" {
		t.Errorf("Audience = %v, want [adk-agent]", got.Audience)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
	tokenStr := signToken(t, privKey, claims)

	_, err = verifier.Verify(tokenStr)
	if err == nil {
		t.Fatal("Verify should have returned an error for expired token")
	}
}

func TestVerify_WrongIssuer(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	claims.Issuer = "wrong-issuer"
	tokenStr := signToken(t, privKey, claims)

	_, err = verifier.Verify(tokenStr)
	if err == nil {
		t.Fatal("Verify should have returned an error for wrong issuer")
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	claims.Audience = jwt.ClaimStrings{"wrong-audience"}
	tokenStr := signToken(t, privKey, claims)

	_, err = verifier.Verify(tokenStr)
	if err == nil {
		t.Fatal("Verify should have returned an error for wrong audience")
	}
}

func TestVerify_MissingUserID(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	claims.UserID = ""
	tokenStr := signToken(t, privKey, claims)

	_, err = verifier.Verify(tokenStr)
	if err == nil {
		t.Fatal("Verify should have returned an error for missing user_id")
	}
}

func TestVerify_MissingChannel(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	claims.Channel = ""
	tokenStr := signToken(t, privKey, claims)

	_, err = verifier.Verify(tokenStr)
	if err == nil {
		t.Fatal("Verify should have returned an error for missing channel")
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	pubPath, privKey := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	claims := validClaims()
	tokenStr := signToken(t, privKey, claims)

	var extractedClaims *Claims
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedClaims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := verifier.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if extractedClaims == nil {
		t.Fatal("claims not found in context")
	}
	if extractedClaims.UserID != "919876543210" {
		t.Errorf("UserID = %q, want %q", extractedClaims.UserID, "919876543210")
	}
	if extractedClaims.Channel != "whatsapp" {
		t.Errorf("Channel = %q, want %q", extractedClaims.Channel, "whatsapp")
	}
}

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	pubPath, _ := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not have been called")
	})

	handler := verifier.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	pubPath, _ := generateTestKeys(t)
	verifier, err := NewJWTVerifier(pubPath, "whatsadk-gateway", "adk-agent")
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not have been called")
	})

	handler := verifier.Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.string")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
