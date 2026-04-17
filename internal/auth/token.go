package auth

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/ethanefung/mail/internal/provider"
)

// GetToken returns a usable access token for email. It checks the keyring
// first, silently refreshes an expired token when a refresh token is
// available, and falls back to the interactive browser flow only when no
// cached token exists or refresh fails.
func GetToken(ctx context.Context, email string, prov *provider.Provider) (*oauth2.Token, error) {
	// Step 1: Load cached token.
	tok, err := LoadToken(email)
	if err != nil {
		return nil, err
	}

	// Step 2: If valid (with 60s skew buffer), return immediately.
	if tok != nil && time.Now().Add(60*time.Second).Before(tok.Expiry) {
		return tok, nil
	}

	// Step 3: If expired but has a refresh token, attempt silent refresh.
	if tok != nil && tok.RefreshToken != "" {
		newTok, err := refreshToken(ctx, prov, tok)
		if err == nil {
			if err := SaveToken(email, newTok); err != nil {
				return nil, fmt.Errorf("persist refreshed token: %w", err)
			}
			return newTok, nil
		}
		// Refresh failed — fall through to browser flow.
	}

	// Step 4: Delete stale entry (if any), then browser flow.
	if tok != nil {
		if err := DeleteToken(email); err != nil {
			return nil, fmt.Errorf("clear expired token: %w", err)
		}
	}

	tok, err = BrowserFlow(ctx, prov)
	if err != nil {
		return nil, err
	}
	if err := SaveToken(email, tok); err != nil {
		return nil, fmt.Errorf("persist token: %w", err)
	}
	return tok, nil
}

// refreshToken uses the stored refresh token to obtain a new access token
// without user interaction. It constructs a minimal oauth2.Config (no
// Scopes or RedirectURL — they are unused for refresh grants) and delegates
// to the oauth2 library's built-in refresh mechanism.
func refreshToken(ctx context.Context, prov *provider.Provider, tok *oauth2.Token) (*oauth2.Token, error) {
	id, secret := MustLoad()

	cfg := &oauth2.Config{
		ClientID:     id,
		ClientSecret: secret,
		Endpoint:     prov.Endpoint,
	}

	newTok, err := cfg.TokenSource(ctx, tok).Token()
	if err != nil {
		return nil, err
	}

	// Preserve the original refresh token when the provider does not
	// return a new one (Google's typical behavior).
	if newTok.RefreshToken == "" {
		newTok.RefreshToken = tok.RefreshToken
	}

	return newTok, nil
}
