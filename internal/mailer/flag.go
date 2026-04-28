package mailer

import (
	"context"
	"fmt"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/oauth2"

	"github.com/ethanefung/ehaul/internal/auth"
	"github.com/ethanefung/ehaul/internal/provider"
)

// aliasToFlag maps human-readable aliases to IMAP system flags.
var aliasToFlag = map[string]imap.Flag{
	"seen":     imap.FlagSeen,
	"answered": imap.FlagAnswered,
	"flagged":  imap.FlagFlagged,
	"deleted":  imap.FlagDeleted,
	"draft":    imap.FlagDraft,
}

// ResolveFlag returns the IMAP flag for a known alias, or passes the alias
// through literally to support custom IMAP keywords.
func ResolveFlag(alias string) imap.Flag {
	if f, ok := aliasToFlag[alias]; ok {
		return f
	}
	return imap.Flag(alias)
}

// FlagParams holds the inputs for FlagMessages.
type FlagParams struct {
	Email          string
	Provider       *provider.Provider
	Token          *oauth2.Token
	Op             string // "add", "rm", "remove", "set"
	Flag           imap.Flag
	UIDs           []imap.UID
	CachedValidity uint32
	CachedOK       bool
}

// FlagMessages connects to the IMAP server, validates UIDVALIDITY against the
// cached value, and issues a UID STORE command to alter flags on the specified
// messages.
func FlagMessages(ctx context.Context, p *FlagParams) error {
	if len(p.UIDs) == 0 {
		return fmt.Errorf("flag: no UIDs provided")
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
		return fmt.Errorf("UIDVALIDITY changed (cached %d, server %d); UIDs may be stale — re-run 'ehaul list' to refresh",
			p.CachedValidity, sel.UIDValidity)
	}

	storeOp, err := mapStoreOp(p.Op)
	if err != nil {
		return err
	}

	uidSet := imap.UIDSetNum(p.UIDs...)
	storeFlags := &imap.StoreFlags{
		Op:     storeOp,
		Silent: false,
		Flags:  []imap.Flag{p.Flag},
	}

	if err := client.Store(uidSet, storeFlags, nil).Close(); err != nil {
		return fmt.Errorf("uid store: %w", err)
	}

	return nil
}

// mapStoreOp converts a user-facing operation string to the corresponding
// imap.StoreFlagsOp constant.
func mapStoreOp(op string) (imap.StoreFlagsOp, error) {
	switch op {
	case "add":
		return imap.StoreFlagsAdd, nil
	case "rm", "remove":
		return imap.StoreFlagsDel, nil
	case "set":
		return imap.StoreFlagsSet, nil
	default:
		return 0, fmt.Errorf("unknown operation %q (must be add, rm, remove, or set)", op)
	}
}
