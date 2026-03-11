package protocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// CanonicalSignPayload builds the canonical bytes for device signature
// verification. The payload is a deterministic JSON object with sorted keys.
func CanonicalSignPayload(deviceID, clientID, clientMode, role string, scopes []string, signedAtMs int64, token, nonce string) []byte {
	m := map[string]any{
		"deviceId":   deviceID,
		"clientId":   clientID,
		"clientMode": clientMode,
		"role":       role,
		"scopes":     scopes,
		"signedAtMs": signedAtMs,
	}
	if token != "" {
		m["token"] = token
	} else {
		m["token"] = nil
	}
	if nonce != "" {
		m["nonce"] = nonce
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(orderedEntries, len(keys))
	for i, k := range keys {
		ordered[i] = orderedEntry{Key: k, Value: m[k]}
	}

	data, _ := json.Marshal(ordered)
	return data
}

// orderedEntry is a key-value pair that serialises as a JSON object field.
type orderedEntry struct {
	Key   string
	Value any
}

func (e orderedEntry) MarshalJSON() ([]byte, error) {
	key, err := json.Marshal(e.Key)
	if err != nil {
		return nil, err
	}
	val, err := json.Marshal(e.Value)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, len(key)+1+len(val))
	buf = append(buf, key...)
	buf = append(buf, ':')
	buf = append(buf, val...)
	return buf, nil
}

type orderedEntries []orderedEntry

func (o orderedEntries) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}
	for i, e := range o {
		if i > 0 {
			buf = append(buf, ',')
		}
		entry, err := e.MarshalJSON()
		if err != nil {
			return nil, err
		}
		buf = append(buf, entry...)
	}
	buf = append(buf, '}')
	return buf, nil
}

// VerifyDeviceSignature verifies an Ed25519 signature against the canonical payload.
func VerifyDeviceSignature(publicKeyB64URL, signatureB64URL string, payload []byte) error {
	pubBytes, err := Base64URLDecode(publicKeyB64URL)
	if err != nil {
		return fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}

	sig, err := Base64URLDecode(signatureB64URL)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}

	if !ed25519.Verify(ed25519.PublicKey(pubBytes), payload, sig) {
		return errors.New("signature verification failed")
	}
	return nil
}

// Base64URLDecode decodes a base64url string (no padding).
func Base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

// Base64URLEncode encodes bytes as base64url (no padding).
func Base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
