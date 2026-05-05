package mailer

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// selfSignedTLSConfig returns a TLS config with a self-signed certificate
// suitable for testing over net.Pipe.
func selfSignedTLSConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
}

// tlsPipe creates a connected pair of TLS connections over net.Pipe.
// Returns (client *tls.Conn, server net.Conn for raw reading/writing).
func tlsPipe(t *testing.T) (*tls.Conn, net.Conn) {
	t.Helper()

	cfg := selfSignedTLSConfig(t)
	clientRaw, serverRaw := net.Pipe()

	serverTLS := tls.Server(serverRaw, cfg)
	go func() {
		_ = serverTLS.Handshake()
	}()

	clientTLS := tls.Client(clientRaw, &tls.Config{InsecureSkipVerify: true})
	if err := clientTLS.Handshake(); err != nil {
		t.Fatal(err)
	}

	return clientTLS, serverTLS
}

func TestWriteLiteral(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"plain query", "from:alice subject:hello"},
		{"backslash at end", `from:alice\`},
		{"double quotes", `"exact phrase"`},
		{"braces", `subject:{important}`},
		{"mixed special chars", `from:alice "test\" {42}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientTLS, serverConn := tlsPipe(t)
			defer clientTLS.Close()
			defer serverConn.Close()

			c := &rawIMAP{
				conn:    clientTLS,
				reader:  textproto.NewReader(bufio.NewReader(clientTLS)),
				nextTag: 1,
			}

			serverReader := bufio.NewReader(serverConn)
			errCh := make(chan error, 1)

			go func() {
				// Server side: read the command line with {N}, send +
				// continuation, then read the literal payload.
				_, err := serverReader.ReadString('\n')
				if err != nil {
					errCh <- err
					return
				}

				// Send continuation response
				_, err = serverConn.Write([]byte("+ go ahead\r\n"))
				if err != nil {
					errCh <- err
					return
				}

				// Read literal payload
				payload, err := serverReader.ReadString('\n')
				if err != nil {
					errCh <- err
					return
				}
				payload = strings.TrimRight(payload, "\r\n")

				if payload != tt.query {
					errCh <- &wireError{got: payload, want: tt.query}
					return
				}

				errCh <- nil
			}()

			// Client side: write the command line with {N}, then writeLiteral
			err := c.writeLine("a001 UID SEARCH X-GM-RAW " + "{" + fmt.Sprintf("%d", len(tt.query)) + "}")
			if err != nil {
				t.Fatalf("writeLine: %v", err)
			}

			err = c.writeLiteral(tt.query)
			if err != nil {
				t.Fatalf("writeLiteral: %v", err)
			}

			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriteLiteralRejectsNonContinuation(t *testing.T) {
	clientTLS, serverConn := tlsPipe(t)
	defer clientTLS.Close()
	defer serverConn.Close()

	c := &rawIMAP{
		conn:    clientTLS,
		reader:  textproto.NewReader(bufio.NewReader(clientTLS)),
		nextTag: 1,
	}

	// Server responds with BAD instead of +
	go func() {
		_, _ = serverConn.Write([]byte("a001 BAD invalid\r\n"))
	}()

	err := c.writeLiteral("test query")
	if err == nil {
		t.Fatal("expected error when server does not send continuation")
	}
	if !strings.Contains(err.Error(), "expected continuation") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUidSearchRawWireFormat(t *testing.T) {
	query := `from:alice "exact\" {42}`

	clientTLS, serverConn := tlsPipe(t)
	defer clientTLS.Close()
	defer serverConn.Close()

	c := &rawIMAP{
		conn:    clientTLS,
		reader:  textproto.NewReader(bufio.NewReader(clientTLS)),
		nextTag: 1,
	}

	serverReader := bufio.NewReader(serverConn)
	type result struct {
		cmdLine string
		payload string
		err     error
	}
	resCh := make(chan result, 1)

	go func() {
		var r result

		// Read command line
		line, err := serverReader.ReadString('\n')
		if err != nil {
			r.err = err
			resCh <- r
			return
		}
		r.cmdLine = strings.TrimRight(line, "\r\n")

		// Send continuation
		_, err = serverConn.Write([]byte("+ ready\r\n"))
		if err != nil {
			r.err = err
			resCh <- r
			return
		}

		// Read literal payload
		line, err = serverReader.ReadString('\n')
		if err != nil {
			r.err = err
			resCh <- r
			return
		}
		r.payload = strings.TrimRight(line, "\r\n")

		// Send search response
		_, err = serverConn.Write([]byte("* SEARCH 100 200 300\r\n"))
		if err != nil {
			r.err = err
			resCh <- r
			return
		}
		_, err = serverConn.Write([]byte("a001 OK SEARCH completed\r\n"))
		if err != nil {
			r.err = err
			resCh <- r
			return
		}

		resCh <- r
	}()

	uids, err := c.uidSearchRaw(query)
	if err != nil {
		t.Fatalf("uidSearchRaw: %v", err)
	}

	r := <-resCh
	if r.err != nil {
		t.Fatalf("server: %v", r.err)
	}

	// Verify command line uses literal syntax with correct byte count.
	wantCmd := "a001 UID SEARCH X-GM-RAW {" + fmt.Sprintf("%d", len(query)) + "}"
	if r.cmdLine != wantCmd {
		t.Errorf("command line:\n  got:  %s\n  want: %s", r.cmdLine, wantCmd)
	}

	// Verify payload is the raw query, not quoted.
	if r.payload != query {
		t.Errorf("literal payload:\n  got:  %s\n  want: %s", r.payload, query)
	}

	// Verify UIDs parsed correctly.
	if len(uids) != 3 {
		t.Fatalf("expected 3 UIDs, got %d", len(uids))
	}
	for i, want := range []uint32{100, 200, 300} {
		if uint32(uids[i]) != want {
			t.Errorf("uid[%d] = %d, want %d", i, uids[i], want)
		}
	}
}

type wireError struct {
	got, want string
}

func (e *wireError) Error() string {
	return "payload mismatch: got " + e.got + ", want " + e.want
}

