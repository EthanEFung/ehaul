package auth

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"github.com/ethanefung/mail/internal/provider"
)

// GetToken returns a usable access token for email, preferring a cached
// keyring entry and falling back to the interactive browser flow when none
// exists or the cached token has expired. Token refresh is not implemented in
// Step 1; expired tokens trigger a full re-auth.
func GetToken(ctx context.Context, email string, prov *provider.Provider) (*oauth2.Token, error) {
	tok, err := LoadToken(email)
	if err != nil {
		return nil, err
	}
	if tok != nil && time.Now().Add(60*time.Second).Before(tok.Expiry) {
		return tok, nil
	}
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
