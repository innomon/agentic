package auth

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GnogentClaims struct {
	UserID  string `json:"userId"`
	Channel string `json:"channel"`
	Domain  string `json:"domain"`
	IP      string `json:"ip"`
	jwt.RegisteredClaims
}

var cachedPublicKey *rsa.PublicKey

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := jwt.ParseRSAPublicKeyFromPEM(b)
	if err != nil {
		return nil, err
	}
	cachedPublicKey = key
	return key, nil
}

func VerifyToken(rawToken string) (*GnogentClaims, error) {
	if cachedPublicKey == nil {
		return nil, fmt.Errorf("public key not loaded; call LoadPublicKey first")
	}
	token, err := jwt.ParseWithClaims(rawToken, &GnogentClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return cachedPublicKey, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*GnogentClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func SignToken(userID, channel string) (string, error) {
	keyBytes, err := os.ReadFile("certs/private.pem")
	if err != nil {
		return "", fmt.Errorf("failed to read private key: %v", err)
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}
	claims := GnogentClaims{
		UserID:  userID,
		Channel: channel,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(key)
}

func GenerateInternalTestToken(userID string) string {
	t, _ := SignToken(userID, "internal-test")
	return t
}
