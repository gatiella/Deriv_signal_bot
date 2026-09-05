package store

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const sessionTTL = 30 * 24 * time.Hour

// NewSessionID generates a random, unguessable session identifier.
func NewSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession creates a session row for userID and returns its id (to be
// stored in a cookie).
func (s *Store) CreateSession(userID int64) (string, error) {
	id, err := NewSessionID()
	if err != nil {
		return "", err
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, time.Now().Add(sessionTTL),
	)
	if err != nil {
		return "", err
	}
	return id, nil
}

// UserIDForSession resolves a session cookie to a user id, if valid and not
// expired. ok is false if the session doesn't exist or has expired.
func (s *Store) UserIDForSession(sessionID string) (userID int64, ok bool) {
	var expiresAt time.Time
	err := s.db.QueryRow(
		`SELECT user_id, expires_at FROM sessions WHERE id = $1`, sessionID,
	).Scan(&userID, &expiresAt)
	if err != nil {
		return 0, false
	}
	if time.Now().After(expiresAt) {
		return 0, false
	}
	return userID, true
}

// DeleteSession logs a user out.
func (s *Store) DeleteSession(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}
