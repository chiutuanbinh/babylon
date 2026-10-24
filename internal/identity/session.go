package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Session struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
}

func (s Session) Expired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

// HashToken returns the hex SHA-256 of a session token. Only the hash is ever
// stored, so a database leak does not leak live sessions.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
