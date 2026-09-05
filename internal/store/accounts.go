package store

// DerivAccount is one connected Deriv login (demo or real) belonging to a user.
type DerivAccount struct {
	ID          int64
	UserID      int64
	LoginID     string
	Currency    string
	IsVirtual   bool
	APITokenEnc string // encrypted; decrypt with cryptoutil.Box before use
}

// UpsertDerivAccount stores (or updates) a connected account. The token is
// expected to already be encrypted by the caller.
func (s *Store) UpsertDerivAccount(userID int64, loginID, currency string, isVirtual bool, encryptedToken string) error {
	_, err := s.db.Exec(`
		INSERT INTO deriv_accounts (user_id, loginid, currency, is_virtual, api_token_enc)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, loginid)
		DO UPDATE SET currency = EXCLUDED.currency, is_virtual = EXCLUDED.is_virtual, api_token_enc = EXCLUDED.api_token_enc
	`, userID, loginID, currency, isVirtual, encryptedToken)
	return err
}

// ListDerivAccounts returns every account a user has connected.
func (s *Store) ListDerivAccounts(userID int64) ([]DerivAccount, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, loginid, currency, is_virtual, api_token_enc
		FROM deriv_accounts WHERE user_id = $1 ORDER BY connected_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DerivAccount
	for rows.Next() {
		var a DerivAccount
		if err := rows.Scan(&a.ID, &a.UserID, &a.LoginID, &a.Currency, &a.IsVirtual, &a.APITokenEnc); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
