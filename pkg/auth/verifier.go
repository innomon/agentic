package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/a2aproject/a2a-go/v2/a2asrv"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID  string `json:"user_id"`
	Channel string `json:"channel"`
	jwt.RegisteredClaims
}

type claimsContextKey struct{}

type JWTVerifier struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
}

func NewJWTVerifier(publicKeyPath, issuer, audience string) (*JWTVerifier, error) {
	keyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not RSA")
	}

	return &JWTVerifier{
		publicKey: rsaPub,
		issuer:    issuer,
		audience:  audience,
	}, nil
}

func (v *JWTVerifier) Verify(tokenStr string) (*Claims, error) {
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
	}
	if v.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.issuer))
	}
	if v.audience != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.audience))
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return v.publicKey, nil
	}, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	if claims.UserID == "" || claims.Channel == "" {
		return nil, fmt.Errorf("missing user_id or channel claim")
	}

	return claims, nil
}

func isLocalhost(r *http.Request) bool {
	var host string
	if r != nil {
		host = r.Host
	} else {
		return true // Assume localhost if no request (A2A local calls)
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func (v *JWTVerifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("BYPASS_AUTH") == "true" && isLocalhost(r) {
			claims := &Claims{
				UserID:  "local-dev",
				Channel: "local",
			}
			ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := v.Verify(tokenStr)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), claimsContextKey{}, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (v *JWTVerifier) MiddlewareMux(next http.Handler) http.Handler {
	return v.Middleware(next)
}

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsContextKey{}).(*Claims)
	return claims
}

type JWTInterceptor struct {
	Verifier *JWTVerifier
}

func (i *JWTInterceptor) Before(ctx context.Context, callCtx *a2asrv.CallContext, req *a2asrv.Request) (context.Context, any, error) {
	if os.Getenv("BYPASS_AUTH") == "true" {
		callCtx.User = &a2asrv.User{
			Name:          "local-dev",
			Authenticated: true,
		}
		return ctx, nil, nil
	}

	authHeader, ok := callCtx.ServiceParams().Get("Authorization")
	if !ok || len(authHeader) == 0 || !strings.HasPrefix(authHeader[0], "Bearer ") {
		return ctx, nil, fmt.Errorf("missing or invalid Authorization header")
	}

	tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
	claims, err := i.Verifier.Verify(tokenStr)
	if err != nil {
		return ctx, nil, err
	}

	callCtx.User = &a2asrv.User{
		Name:          claims.UserID,
		Authenticated: true,
	}

	return context.WithValue(ctx, claimsContextKey{}, claims), nil, nil
}

func (i *JWTInterceptor) After(ctx context.Context, callCtx *a2asrv.CallContext, resp *a2asrv.Response) error {
	return nil
}
