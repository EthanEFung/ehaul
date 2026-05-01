// Package provider maps an email address's domain to the IMAP and OAuth2
// configuration required to talk to that email provider.
package provider

import (
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Provider holds the configuration needed to connect to and authenticate
// against a single email provider.
type Provider struct {
	Name        string
	IMAPHost    string
	IMAPPort    int
	OAuthScopes []string
	Endpoint    oauth2.Endpoint
}

var gmail = &Provider{
	Name:        "gmail",
	IMAPHost:    "imap.gmail.com",
	IMAPPort:    993,
	OAuthScopes: []string{"https://mail.google.com/"},
	Endpoint:    google.Endpoint,
}

var registry = map[string]*Provider{
	"gmail.com":      gmail,
	"googlemail.com": gmail,
}

var nameRegistry = map[string]*Provider{
	"gmail": gmail,
}

// LookupByName returns the Provider registered under the given name.
// Use this when the caller has an explicit --provider override.
func LookupByName(name string) (*Provider, error) {
	p, ok := nameRegistry[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return p, nil
}

// Lookup returns the Provider for the given email address's domain.
// Unknown domains yield an error; Google Workspace custom domains are not
// supported in v1 and will fail here.
func Lookup(email string) (*Provider, error) {
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return nil, fmt.Errorf("invalid email address: %q", email)
	}
	domain := strings.ToLower(email[at+1:])
	p, ok := registry[domain]
	if !ok {
		return nil, fmt.Errorf("unsupported provider for domain %q", domain)
	}
	return p, nil
}
