package auth

// Verifier validates authentication credentials (tokens and passwords).
type Verifier struct {
	tokens        map[string]struct{}
	password      string
	allowPassword bool
}

// NewVerifier creates a new auth verifier.
func NewVerifier(tokens []string, password string, allowPassword bool) *Verifier {
	m := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		m[t] = struct{}{}
	}
	return &Verifier{
		tokens:        m,
		password:      password,
		allowPassword: allowPassword,
	}
}

// VerifyToken checks whether the given token is in the allowed set.
func (v *Verifier) VerifyToken(token string) bool {
	_, ok := v.tokens[token]
	return ok
}

// VerifyPassword checks whether the given password matches.
func (v *Verifier) VerifyPassword(pw string) bool {
	if !v.allowPassword {
		return false
	}
	return v.password != "" && v.password == pw
}
