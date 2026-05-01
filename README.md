# ehaul

A terminal-native Gmail client built for batch inbox operations. ehaul connects to Gmail over IMAP with OAuth2 and outputs clean, tab-delimited text designed to compose with standard Unix tools. List and search to build a candidate set, pipe through `grep`, `cut`, `sort`, and `awk` to narrow it down, then flag or move the exact UIDs you reviewed. Inbox cleanup as a text-processing problem.

## Setup

### Prerequisites

- Go 1.25+
- A Google Cloud project with the Gmail API enabled

### Create OAuth credentials

ehaul uses OAuth2 to access Gmail over IMAP. OAuth2 has three parties: the **provider** (Google), the **application** (ehaul), and the **user** (you). The provider needs to know which application is requesting access, so every application registers itself and receives a **client ID** and **client secret**. These identify the application, not the user.

This project does not ship a shared client ID. Each person who builds ehaul creates their own OAuth application in Google Cloud. This is intentional:

- A client ID/secret pair is an **application identity**. If it were committed to a public repo, anyone could impersonate the application and Google could revoke it.
- Google enforces **per-application rate limits**. A shared client ID across many users can hit quota walls.
- Google requires apps that use a shared client ID to go through a **verification review** (privacy policy, homepage, branding check). That process is tied to a single maintainer's account and doesn't make sense for a personal CLI tool.

By creating your own Google Cloud project, you control your own quota, your own consent screen, and your own credentials. The `.credentials` file is gitignored so they never leave your machine.

To create your credentials:

1. Go to [Google Cloud Console](https://console.cloud.google.com/) > APIs & Services > Credentials
2. Create an OAuth 2.0 Client ID (application type: **Desktop app**)
3. Copy the Client ID and Client Secret

### Build

```sh
cp .credentials.example .credentials
# Edit .credentials and fill in GMAIL_CLIENT_ID and GMAIL_CLIENT_SECRET
make install
```

This produces an `ehaul` binary with your credentials embedded at compile time.

### First run and the consent screen

On first run, ehaul opens your browser to Google's OAuth consent screen. Here's what happens and why:

1. **"Google hasn't verified this app"** — You will see a warning screen saying Google doesn't recognize the app. This is expected. Your Google Cloud project is new and unverified. Google shows this for any app that hasn't gone through their review process. Click **Advanced** → **Go to \<your-project-name\> (unsafe)** to continue. "Unsafe" means unreviewed by Google, not that the app is malicious.

2. **Permission grant** — Google asks whether you want to grant the application access to your Gmail. This is the OAuth consent step. You are authorizing your own build of ehaul (identified by the client ID you created) to read and modify your email. The scope requested is `https://mail.google.com/`, which is full IMAP access.

3. **Redirect back** — After you approve, Google redirects to a local server that ehaul started on `127.0.0.1`. This is the standard "installed app" OAuth flow — the authorization code never leaves your machine. ehaul exchanges the code for an access token and a refresh token.

4. **Token storage** — The tokens are saved in your OS keyring (macOS Keychain, Linux Secret Service, Windows Credential Manager). On subsequent runs, ehaul loads the token from the keyring and refreshes it automatically. You won't see the browser again unless you revoke access or the refresh token expires.

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
ehaul flag user@gmail.com rm seen 1234
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

## Google Workspace / custom domains

By default, ehaul detects the provider from the email domain (`gmail.com`, `googlemail.com`). If you use Gmail with a custom domain (e.g. Google Workspace), pass `--provider=gmail` to override detection:

```sh
ehaul list --provider=gmail user@acme.com
ehaul flag --provider=gmail user@acme.com add seen 1234
ehaul move --provider=gmail user@acme.com Archive 1234
ehaul folders --provider=gmail user@acme.com
```

The `--provider` flag is available on all subcommands.

## Supported providers

| Provider | Domains | `--provider` name |
|----------|---------|-------------------|
| Gmail | `gmail.com`, `googlemail.com` | `gmail` |

Currently looking for contributors to add support for other providers! The main work is implementing the provider-specific authentication flow and any IMAP extensions they require. Please open an issue if you'd like to help out.
