package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"golang.org/x/oauth2"

	"github.com/ethanefung/ehaul/internal/auth"
	"github.com/ethanefung/ehaul/internal/provider"
)

// GmailSearch performs a Gmail X-GM-RAW search, then fetches the matching
// message headers using the normal go-imap/v2 client path.
func GmailSearch(ctx context.Context, email string, prov *provider.Provider, tok *oauth2.Token, query string, limit, page int) ([]Header, uint32, error) {
	if limit <= 0 || page <= 0 {
		return nil, 0, nil
	}
	if strings.ContainsAny(query, "\r\n") {
		return nil, 0, fmt.Errorf("--gm-search query must not contain line breaks")
	}

	addr := fmt.Sprintf("%s:%d", prov.IMAPHost, prov.IMAPPort)
	uids, err := gmRawSearch(ctx, addr, email, tok.AccessToken, query)
	if err != nil {
		return nil, 0, err
	}
	if len(uids) == 0 {
		return nil, 0, nil
	}

	sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
	offset := (page - 1) * limit
	if offset >= len(uids) {
		return nil, 0, nil
	}
	end := offset + limit
	if end > len(uids) {
		end = len(uids)
	}

	return fetchHeaders(ctx, email, prov, tok, uids[offset:end])
}

// gmRawSearch uses a raw TLS IMAP connection to issue Gmail's X-GM-RAW search
// extension, which go-imap/v2 cannot express with SearchCriteria.
func gmRawSearch(ctx context.Context, addr, email, accessToken, query string) ([]imap.UID, error) {
	conn, err := dialRawIMAP(ctx, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	defer func() { _ = conn.logout() }()

	if err := conn.authenticateXOAUTH2(email, accessToken); err != nil {
		return nil, err
	}
	if err := conn.requireGMExtension(); err != nil {
		return nil, err
	}
	if err := conn.selectInbox(); err != nil {
		return nil, err
	}

	return conn.uidSearchRaw(query)
}

// fetchHeaders dials the mailbox over the normal IMAP client path, selects
// INBOX, fetches the specified UIDs, and converts them to Header values.
func fetchHeaders(ctx context.Context, email string, prov *provider.Provider, tok *oauth2.Token, uids []imap.UID) ([]Header, uint32, error) {
	if len(uids) == 0 {
		return nil, 0, nil
	}

	addr := fmt.Sprintf("%s:%d", prov.IMAPHost, prov.IMAPPort)
	client, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("dial %s: %w", addr, err)
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
		return nil, 0, fmt.Errorf("imap authenticate: %w", err)
	}
	defer func() { _ = client.Logout().Wait() }()

	sel, err := client.Select("INBOX", nil).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("select INBOX: %w", err)
	}
	if sel.NumMessages == 0 {
		return nil, sel.UIDValidity, nil
	}

	opts := &imap.FetchOptions{
		UID:          true,
		Envelope:     true,
		InternalDate: true,
	}

	uidSet := imap.UIDSetNum(uids...)
	msgs, err := client.Fetch(uidSet, opts).Collect()
	if err != nil {
		return nil, 0, fmt.Errorf("fetch headers: %w", err)
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
	return out, sel.UIDValidity, nil
}

type rawIMAP struct {
	conn     *tls.Conn
	reader   *textproto.Reader
	deadline time.Time
	nextTag  int
}

func dialRawIMAP(ctx context.Context, addr string) (*rawIMAP, error) {
	dialer := &net.Dialer{}
	var deadline time.Time
	if d, ok := ctx.Deadline(); ok {
		dialer.Deadline = d
		deadline = d
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, nil)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	imapConn := &rawIMAP{
		conn:     conn,
		reader:   textproto.NewReader(bufio.NewReader(conn)),
		deadline: deadline,
		nextTag:  1,
	}
	if err := imapConn.applyDeadline(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	line, err := imapConn.readLine()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read greeting: %w", err)
	}
	if !strings.HasPrefix(line, "* OK") {
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected IMAP greeting: %s", line)
	}

	return imapConn, nil
}

func (c *rawIMAP) Close() error {
	return c.conn.Close()
}

func (c *rawIMAP) applyDeadline() error {
	if c.deadline.IsZero() {
		return nil
	}
	return c.conn.SetDeadline(c.deadline)
}

func (c *rawIMAP) readLine() (string, error) {
	if err := c.applyDeadline(); err != nil {
		return "", err
	}
	return c.reader.ReadLine()
}

func (c *rawIMAP) writeLine(line string) error {
	if err := c.applyDeadline(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(c.conn, "%s\r\n", line)
	return err
}

func (c *rawIMAP) nextTagString() string {
	tag := fmt.Sprintf("a%03d", c.nextTag)
	c.nextTag++
	return tag
}

func (c *rawIMAP) authenticateXOAUTH2(email, accessToken string) error {
	ir, err := xoauth2InitialResponse(email, accessToken)
	if err != nil {
		return err
	}

	tag := c.nextTagString()
	if err := c.writeLine(fmt.Sprintf("%s AUTHENTICATE XOAUTH2 %s", tag, ir)); err != nil {
		return fmt.Errorf("auth XOAUTH2: %w", err)
	}

	aborted := false
	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("auth XOAUTH2: %w", err)
		}
		switch {
		case strings.HasPrefix(line, "+"):
			if !aborted {
				if err := c.writeLine(""); err != nil {
					return fmt.Errorf("auth XOAUTH2: %w", err)
				}
				aborted = true
			}
		case strings.HasPrefix(line, tag+" OK"):
			return nil
		case strings.HasPrefix(line, tag+" NO"), strings.HasPrefix(line, tag+" BAD"):
			return fmt.Errorf("auth XOAUTH2: %s", line)
		}
	}
}

func (c *rawIMAP) requireGMExtension() error {
	tag := c.nextTagString()
	if err := c.writeLine(fmt.Sprintf("%s CAPABILITY", tag)); err != nil {
		return fmt.Errorf("capability: %w", err)
	}

	hasGMExtension := false
	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("capability: %w", err)
		}
		switch {
		case strings.HasPrefix(line, "* CAPABILITY"):
			for _, tok := range strings.Fields(line)[2:] {
				if strings.EqualFold(tok, "X-GM-EXT-1") {
					hasGMExtension = true
				}
			}
		case strings.HasPrefix(line, tag+" OK"):
			if !hasGMExtension {
				return fmt.Errorf("server does not support X-GM-EXT-1 extension")
			}
			return nil
		case strings.HasPrefix(line, tag+" NO"), strings.HasPrefix(line, tag+" BAD"):
			return fmt.Errorf("capability: %s", line)
		}
	}
}

func (c *rawIMAP) selectInbox() error {
	tag := c.nextTagString()
	if err := c.writeLine(fmt.Sprintf("%s SELECT INBOX", tag)); err != nil {
		return fmt.Errorf("select INBOX: %w", err)
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return fmt.Errorf("select INBOX: %w", err)
		}
		switch {
		case strings.HasPrefix(line, tag+" OK"):
			return nil
		case strings.HasPrefix(line, tag+" NO"), strings.HasPrefix(line, tag+" BAD"):
			return fmt.Errorf("select INBOX: %s", line)
		}
	}
}

func (c *rawIMAP) writeLiteral(data string) error {
	line, err := c.readLine()
	if err != nil {
		return fmt.Errorf("literal continuation: %w", err)
	}
	if !strings.HasPrefix(line, "+") {
		return fmt.Errorf("expected continuation, got: %s", line)
	}
	return c.writeLine(data)
}

func (c *rawIMAP) uidSearchRaw(query string) ([]imap.UID, error) {
	tag := c.nextTagString()
	if err := c.writeLine(fmt.Sprintf("%s UID SEARCH X-GM-RAW {%d}", tag, len(query))); err != nil {
		return nil, fmt.Errorf("uid search: %w", err)
	}
	if err := c.writeLiteral(query); err != nil {
		return nil, fmt.Errorf("uid search: %w", err)
	}

	var uids []imap.UID
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, fmt.Errorf("uid search: %w", err)
		}
		switch {
		case strings.HasPrefix(line, "* SEARCH"):
			fields := strings.Fields(line)
			for _, field := range fields[2:] {
				n, err := strconv.ParseUint(field, 10, 32)
				if err != nil {
					return nil, fmt.Errorf("parse SEARCH UID %q: %w", field, err)
				}
				uids = append(uids, imap.UID(uint32(n)))
			}
		case strings.HasPrefix(line, tag+" OK"):
			return uids, nil
		case strings.HasPrefix(line, tag+" NO"), strings.HasPrefix(line, tag+" BAD"):
			return nil, fmt.Errorf("uid search: %s", line)
		}
	}
}

func (c *rawIMAP) logout() error {
	tag := c.nextTagString()
	if err := c.writeLine(fmt.Sprintf("%s LOGOUT", tag)); err != nil {
		return err
	}

	for {
		line, err := c.readLine()
		if err != nil {
			return err
		}
		if strings.HasPrefix(line, tag+" ") {
			return nil
		}
	}
}

func xoauth2InitialResponse(email, accessToken string) (string, error) {
	client := auth.NewXOAUTH2Client(email, accessToken)
	_, ir, err := client.Start()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ir), nil
}
