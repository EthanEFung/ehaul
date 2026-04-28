package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

// keyringService is the fixed service name used for all entries this binary
// writes to the OS keyring.
const keyringService = "ehaul"

// storedToken is the on-disk (keyring) representation of an oauth2.Token.
// Only the fields we actually need are persisted.
type storedToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Expiry       string `json:"expiry"`
	TokenType    string `json:"token_type,omitempty"`
}

// SaveToken persists tok to the OS keyring under the given email.
func SaveToken(email string, tok *oauth2.Token) error {
	st := storedToken{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		Expiry:       tok.Expiry.UTC().Format(time.RFC3339),
		TokenType:    tok.TokenType,
	}
	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	if err := keyring.Set(keyringService, email, string(b)); err != nil {
		return fmt.Errorf("save token to keyring: %w", err)
	}
	return nil
}

// LoadToken retrieves the stored oauth2.Token for email, or (nil, nil) if no
// entry exists. A malformed entry is returned as an error.
func LoadToken(email string) (*oauth2.Token, error) {
	s, err := keyring.Get(keyringService, email)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load token from keyring: %w", err)
	}
	var st storedToken
	if err := json.Unmarshal([]byte(s), &st); err != nil {
		return nil, fmt.Errorf("decode stored token: %w", err)
	}
	expiry, err := time.Parse(time.RFC3339, st.Expiry)
	if err != nil {
		return nil, fmt.Errorf("parse stored token expiry: %w", err)
	}
	return &oauth2.Token{
		AccessToken:  st.AccessToken,
		RefreshToken: st.RefreshToken,
		TokenType:    st.TokenType,
		Expiry:       expiry,
	}, nil
}

// DeleteToken removes the stored token for email. Missing entries are not an
// error.
func DeleteToken(email string) error {
	err := keyring.Delete(keyringService, email)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete token: %w", err)
	}
	return nil
}
