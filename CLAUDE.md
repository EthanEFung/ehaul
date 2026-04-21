# Mail

A simple unix script that lists the recent email headers of a user.

## Libraries

Utilize `rtk go doc` to find documentation for these libraries.

- `github.com/emersion/go-imap/v2`: contains the subdirectory `imapclient` which will be utilized to connect to the provider servers and fetch the email headers.
- `github.com/zalando/go-keyring`: used to securely store oauth tokens for the email providers.

## Providers

The script will support the following email providers:
- Gmail

## Three Man Team
Available agents: Arch (Architect), Bob (Builder), Richard (Reviewer)
