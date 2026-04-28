# ehaul

A command-line email client for Gmail over IMAP. Authenticate with OAuth2, then list, search, flag, and move messages from your terminal.

## Setup

### Prerequisites

- Go 1.25+
- A Google Cloud project with the Gmail API enabled

### Create OAuth credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/) > APIs & Services > Credentials
2. Create an OAuth 2.0 Client ID (application type: **Desktop app**)
3. Copy the Client ID and Client Secret

### Build

```sh
cp .credentials.example .credentials
# Edit .credentials and fill in GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET
make build
```

This produces a `./ehaul` binary with the credentials embedded at compile time.

On first run, your browser will open for Google OAuth consent. The resulting token is stored in your OS keyring and refreshed automatically.

## Usage

### List messages

```sh
ehaul list user@gmail.com
```

Prints tab-delimited rows: `UID  From  Subject  Date`

Options:

| Flag | Default | Description |
|------|---------|-------------|
| `--unread` | false | Show only unread messages |
| `--gm-search="<query>"` | | Gmail search query ([X-GM-RAW syntax](https://support.google.com/mail/answer/7190)) |
| `--limit=N` | 20 | Messages per page |
| `--page=N` | 1 | Page number (1-indexed, most recent first) |

```sh
# Unread messages
ehaul list --unread user@gmail.com

# Gmail search
ehaul list --gm-search="from:alice subject:invoice" user@gmail.com

# Second page, 10 per page
ehaul list --limit=10 --page=2 user@gmail.com
```

### Flag messages

Add, remove, or set flags on messages by UID.

```sh
ehaul flag <email> <operation> <flag> <uid...>
```

- **Operations:** `add`, `rm` (or `remove`), `set`
- **Flags:** `seen`, `flagged`, `answered`, `deleted`, `draft`, or any custom keyword

```sh
# Mark as read
ehaul flag user@gmail.com add seen 1234 1235

# Unflag
ehaul flag user@gmail.com rm flagged 1234
```

### Move messages

Move messages by UID to another mailbox.

```sh
ehaul move <email> <destination-mailbox> <uid...>
```

```sh
ehaul move user@gmail.com "[Gmail]/Trash" 1234 1235
```

Use `ehaul folders` to find valid mailbox names.

### List folders

```sh
ehaul folders user@gmail.com
```

Prints each mailbox name, with IMAP attributes where present.

## How it works

- **Authentication:** OAuth2 with PKCE via a local loopback server. Tokens are persisted in the OS keyring (macOS Keychain, Linux Secret Service, Windows Credential Manager) and refreshed automatically.
- **IMAP:** Uses [go-imap/v2](https://github.com/emersion/go-imap) for standard operations. Gmail-specific search (`X-GM-RAW`) is handled with raw IMAP commands since the library doesn't expose that extension.
- **UIDVALIDITY:** Cached locally under `~/.cache/ehaul` (or `$XDG_CACHE_HOME`). Flag and move commands check cached UIDVALIDITY against the server to catch stale UIDs before modifying anything.
- **Output sanitization:** Control characters in headers are replaced with spaces to prevent terminal injection.

## Supported providers

| Provider | Domains |
|----------|---------|
| Gmail | `gmail.com`, `googlemail.com` |
