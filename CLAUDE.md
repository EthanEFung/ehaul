# Mail

A simple unix script that lists the recent email headers of a user.

## Examples
```bash
go build .

# lists the last 20 emails of the user
./mail list "user@mail.com"

# lists the last 30 emails of the user
./mail list --limit=30 "user@mail.com"

# lists the last 20 emails of the user, starting from the 20th email
./mail list --page=2 "user@mail.com"

# lists the last 20 unread emails
./mail list --unread "user@mail.com"
```

## Libraries

Utilize `go doc` to find documentation for these libraries.

- `github.com/emersion/go-imap/v2`: contains the subdirectory `imapclient` which will be utilized to connect to the provider servers and fetch the email headers.
- `github.com/zalando/go-keyring`: used to securely store oauth tokens for the email providers.

## Providers

The script will support the following email providers:
- Gmail

## Requirements

1. When the user runs the script, a TLS connection to the email provider's server is opened following the IMAP protocol. The script should attempt to connect to the server using the default port (993) and the appropriate server address for the provider (e.g., `imap.gmail.com` for Gmail).
2. If the server requires authentication, the script should look in the secrets manager for the oauth token, if not found, it should open a browser window to authenticate the user
3. If the user is authenticated successfully, the script should select the INBOX mailbox.
4. The script should then fetch the email headers for the specified user, applying any filters (e.g., unread emails) and pagination as requested by the user.

## Three Man Team
Available agents: Arch (Architect), Bob (Builder), Richard (Reviewer)
