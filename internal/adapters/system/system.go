// Package system provides the real clock and token generator for the identity ports.
package system

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// Tokens generates 256-bit random URL-safe tokens.
type Tokens struct{}

func (Tokens) New() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
