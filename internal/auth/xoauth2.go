package auth

import (
	"fmt"

	"github.com/emersion/go-sasl"
)

// xoauth2Client is a sasl.Client implementing the XOAUTH2 mechanism used by
// Gmail's IMAP service. go-sasl does not ship XOAUTH2 out of the box.
//
// Protocol: the initial response is a single formatted string; the server
// either accepts it or issues a challenge containing an error JSON blob, which
// we treat as an auth failure. See:
// https://developers.google.com/gmail/imap/xoauth2-protocol
type xoauth2Client struct {
	username string
	token    string
}

// NewXOAUTH2Client returns a SASL client that authenticates to an IMAP server
// using an OAuth2 bearer access token.
func NewXOAUTH2Client(username, accessToken string) sasl.Client {
	return &xoauth2Client{username: username, token: accessToken}
}

func (c *xoauth2Client) Start() (mech string, ir []byte, err error) {
	payload := fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.username, c.token)
	return "XOAUTH2", []byte(payload), nil
}

func (c *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	// XOAUTH2 is single-shot: any server challenge indicates auth failure and
	// carries an error JSON payload.
	return nil, fmt.Errorf("xoauth2: auth rejected: %q", challenge)
}
