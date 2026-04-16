// Package mailer drives IMAP operations against an authenticated mailbox.
package mailer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/oauth2"

	"github.com/ethanefung/mail/internal/auth"
	"github.com/ethanefung/mail/internal/provider"
)

// Header is the display-worthy subset of an email message's metadata.
type Header struct {
	UID     uint32
	From    string
	Subject string
	Date    time.Time
}

// ListInbox dials the provider's IMAP server over TLS, authenticates with
// XOAUTH2, selects INBOX, and returns the most recent `limit` headers ordered
// newest first. When unread is true, only messages without the \Seen flag are
// returned. It respects the deadline on ctx.
func ListInbox(ctx context.Context, email string, prov *provider.Provider, tok *oauth2.Token, limit int, unread bool) ([]Header, error) {
	if limit <= 0 {
		return nil, nil
	}

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

	sel, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("select INBOX: %w", err)
	}
	if sel.NumMessages == 0 {
		return nil, nil
	}

	opts := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		InternalDate: true,
	}

	var msgs []*imapclient.FetchMessageBuffer
	if unread {
		criteria := &imap.SearchCriteria{
			NotFlag: []imap.Flag{imap.FlagSeen},
		}
		searchData, err := client.UIDSearch(criteria, nil).Wait()
		if err != nil {
			return nil, fmt.Errorf("uid search: %w", err)
		}
		uids := searchData.AllUIDs()
		if len(uids) == 0 {
			return nil, nil
		}
		// Sort descending (highest UID ≈ most recent) and take the first `limit`.
		sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
		if len(uids) > limit {
			uids = uids[:limit]
		}
		uidSet := imap.UIDSetNum(uids...)
		msgs, err = client.Fetch(uidSet, opts).Collect()
		if err != nil {
			return nil, fmt.Errorf("fetch headers: %w", err)
		}
	} else {
		to := sel.NumMessages
		from := uint32(1)
		if to > uint32(limit) {
			from = to - uint32(limit) + 1
		}
		var seqSet imap.SeqSet
		seqSet.AddRange(from, to)
		msgs, err = client.Fetch(seqSet, opts).Collect()
		if err != nil {
			return nil, fmt.Errorf("fetch headers: %w", err)
		}
	}

	out := make([]Header, 0, len(msgs))
	for _, m := range msgs {
		h := Header{
			UID:  uint32(m.UID),
			Date: m.InternalDate,
		}
		if m.Envelope != nil {
			h.Subject = m.Envelope.Subject
			if len(m.Envelope.From) > 0 {
				h.From = formatFrom(m.Envelope.From[0])
			}
		}
		out = append(out, h)
	}

	// Newest first by server arrival time.
	sort.Slice(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	return out, nil
}

// formatFrom returns the display name when present, else the bare email.
// go-imap/v2 has already UTF-8 decoded the Name field.
func formatFrom(a imap.Address) string {
	if strings.TrimSpace(a.Name) != "" {
		return a.Name
	}
	return a.Addr()
}

// Sanitize replaces control characters that could corrupt tab-delimited output
// or be interpreted by the user's terminal emulator (C0 0x00–0x1F, DEL 0x7F,
// C1 0x80–0x9F) with a single space.
func Sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r <= 0x1F || r == 0x7F || (r >= 0x80 && r <= 0x9F) {
			return ' '
		}
		return r
	}, s)
}
