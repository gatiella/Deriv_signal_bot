package store

import "database/sql"

// User is a row from the users table.
type User struct {
	ID    int64
	Email string
}

// FindOrCreateUserByEmail returns the existing user with this email, or
// creates one. Deriv OAuth gives us a verified email on authorize, so we use
// it as the natural identity key - there's no separate password to manage.
func (s *Store) FindOrCreateUserByEmail(email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`SELECT id, email FROM users WHERE email = $1`, email).Scan(&u.ID, &u.Email)
	if err == nil {
		return u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	err = s.db.QueryRow(
		`INSERT INTO users (email) VALUES ($1) RETURNING id, email`, email,
	).Scan(&u.ID, &u.Email)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUser fetches a user by id.
func (s *Store) GetUser(id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`SELECT id, email FROM users WHERE id = $1`, id).Scan(&u.ID, &u.Email)
	if err != nil {
		return nil, err
	}
	return u, nil
}
