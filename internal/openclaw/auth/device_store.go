package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// DeviceToken represents an issued device auth token.
type DeviceToken struct {
	DeviceID   string
	Role       string
	Token      string
	Scopes     []string
	IssuedAtMs int64
}

// DeviceTokenStore is an interface for persisting device tokens.
type DeviceTokenStore interface {
	// Store saves a device token keyed by (deviceID, role).
	Store(token DeviceToken) error
	// Lookup retrieves a device token by (deviceID, role).
	Lookup(deviceID, role string) (*DeviceToken, error)
	// Delete removes a device token by (deviceID, role).
	Delete(deviceID, role string) error
}

// MemoryDeviceTokenStore is an in-memory implementation.
type MemoryDeviceTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]DeviceToken // key: "deviceID:role"
}

// NewMemoryDeviceTokenStore creates a new in-memory device token store.
func NewMemoryDeviceTokenStore() *MemoryDeviceTokenStore {
	return &MemoryDeviceTokenStore{
		tokens: make(map[string]DeviceToken),
	}
}

func deviceKey(deviceID, role string) string {
	return deviceID + ":" + role
}

// Store saves a device token keyed by (deviceID, role).
// If IssuedAtMs is zero, it is set to the current time.
func (s *MemoryDeviceTokenStore) Store(token DeviceToken) error {
	if token.IssuedAtMs == 0 {
		token.IssuedAtMs = time.Now().UnixMilli()
	}
	s.mu.Lock()
	s.tokens[deviceKey(token.DeviceID, token.Role)] = token
	s.mu.Unlock()
	return nil
}

// Lookup retrieves a device token by (deviceID, role).
func (s *MemoryDeviceTokenStore) Lookup(deviceID, role string) (*DeviceToken, error) {
	s.mu.RLock()
	t, ok := s.tokens[deviceKey(deviceID, role)]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	return &t, nil
}

// Delete removes a device token by (deviceID, role).
func (s *MemoryDeviceTokenStore) Delete(deviceID, role string) error {
	s.mu.Lock()
	delete(s.tokens, deviceKey(deviceID, role))
	s.mu.Unlock()
	return nil
}

// GenerateDeviceToken creates a random 32-byte base64url-encoded token.
func GenerateDeviceToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating device token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
