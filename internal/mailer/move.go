package mailer

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/oauth2"

	"github.com/ethanefung/mail/internal/auth"
	"github.com/ethanefung/mail/internal/provider"
)

// MoveParams holds the inputs for MoveMessages.
type MoveParams struct {
	Email          string
	Provider       *provider.Provider
	Token          *oauth2.Token
	Destination    string
	UIDs           []imap.UID
	CachedValidity uint32
	CachedOK       bool
}

// MoveMessages connects to the IMAP server, validates UIDVALIDITY against the
// cached value, and issues a UID MOVE command to relocate the specified
// messages to the destination mailbox.
func MoveMessages(ctx context.Context, p *MoveParams) error {
	if len(p.UIDs) == 0 {
		return fmt.Errorf("move: no UIDs provided")
	}

	addr := fmt.Sprintf("%s:%d", p.Provider.IMAPHost, p.Provider.IMAPPort)
	client, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer client.Close()

	// Surface context cancellation by closing the connection early.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = client.Close()
		case <-stop:
		}
	}()

	if err := client.Authenticate(auth.NewXOAUTH2Client(p.Email, p.Token.AccessToken)); err != nil {
		return fmt.Errorf("imap authenticate: %w", err)
	}
	defer func() { _ = client.Logout().Wait() }()

	sel, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}

	// UIDVALIDITY check (closes KG-4).
	if p.CachedOK && p.CachedValidity != sel.UIDValidity {
		return fmt.Errorf("UIDVALIDITY changed (cached %d, server %d); UIDs may be stale — re-run 'mail list' to refresh",
			p.CachedValidity, sel.UIDValidity)
	}

	uidSet := imap.UIDSetNum(p.UIDs...)
	if _, err := client.Move(uidSet, p.Destination).Wait(); err != nil {
		return fmt.Errorf("uid move to %q: %w", p.Destination, err)
	}

	return nil
}
