package mailer

import (
	"context"
	"fmt"
	"sort"

	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/oauth2"

	"github.com/ethanefung/mail/internal/auth"
	"github.com/ethanefung/mail/internal/provider"
)

// Folder represents a single IMAP mailbox returned by the LIST command.
type Folder struct {
	Name  string
	Attrs []string // string representations of imap.MailboxAttr values
}

// ListFolders dials the provider's IMAP server over TLS, authenticates with
// XOAUTH2, issues LIST "" "*" to enumerate all mailboxes, and returns them
// sorted alphabetically by name. It does not SELECT any mailbox.
func ListFolders(ctx context.Context, email string, prov *provider.Provider, tok *oauth2.Token) ([]Folder, error) {
	addr := fmt.Sprintf("%s:%d", prov.IMAPHost, prov.IMAPPort)
	client, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
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

	if err := client.Authenticate(auth.NewXOAUTH2Client(email, tok.AccessToken)); err != nil {
		return nil, fmt.Errorf("imap authenticate: %w", err)
	}
	defer func() { _ = client.Logout().Wait() }()

	listData, err := client.List("", "*", nil).Collect()
	if err != nil {
		return nil, fmt.Errorf("list mailboxes: %w", err)
	}

	folders := make([]Folder, 0, len(listData))
	for _, ld := range listData {
		attrs := make([]string, len(ld.Attrs))
		for i, a := range ld.Attrs {
			attrs[i] = string(a)
		}
		folders = append(folders, Folder{
			Name:  ld.Mailbox,
			Attrs: attrs,
		})
	}

	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})

	return folders, nil
}
