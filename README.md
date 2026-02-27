# Slacker

A Go CLI wrapper over [slackdump](https://github.com/rusq/slackdump) for quickly grabbing Slack conversations without admin access. Given a thread link, dump the thread. Given two message links (start/end), dump that range. Output goes to stdout as readable text.

## Installation

Requires [Go 1.25+](https://go.dev/dl/).

```bash
git clone https://github.com/matthewvolk/slacker.git
cd slacker
go install .
```

This builds the binary and places it in your `$GOBIN` directory (defaults to `~/go/bin`). Make sure `$GOBIN` is on your `$PATH`:

```bash
# Add to your ~/.zshrc or ~/.bashrc if not already there
export PATH="$PATH:$(go env GOBIN)"
# Or if GOBIN is unset (uses the default):
export PATH="$PATH:$(go env GOPATH)/bin"
```

After source code changes, run `go install .` again from the repo to update the binary in place.

## Usage

```bash
# Thread mode: dump an entire thread
slacker "https://workspace.slack.com/archives/C123ABC456/p1577694990000400"

# Range mode: dump messages between two message links (same channel)
slacker "https://workspace.slack.com/archives/C123ABC456/p1577694990000400" \
        "https://workspace.slack.com/archives/C123ABC456/p1577700000000000"
```

### Output format

```
> Alice Smith (@asmith) @ 26/02/2026 11:53:32 PM CST:
Hey @bjones, check out this thread

|   > Bob Jones (@bjones) @ 26/02/2026 11:54:10 PM CST:
|   Thanks @asmith!
```

- Thread replies are indented with `|   ` prefix
- `<@U123ABC>` mentions are resolved to `@username`
- Message headers show full name, `@username`, and timestamp
- Output goes to stdout, so you can pipe it anywhere:

```bash
# Copy to clipboard (macOS)
slacker "https://..." | pbcopy

# Save to file
slacker "https://..." > thread.txt
```

## Authentication

Slacker checks for credentials in this order:

1. **Environment variables** — `SLACK_TOKEN` and `SLACK_COOKIE` set in your shell
2. **`.env` file** in the current working directory (project-specific override)
3. **Global credentials file** — stored in the OS config directory (see below)
4. **Browser login** — interactive prompt as a last resort

### Browser login (SSO, Google, email/password, etc.)

On first run (or when credentials expire), slacker opens an interactive browser prompt via slackdump's ROD auth, supporting Okta SSO, Google SSO, email/password, and more.

After a successful browser login, credentials are **automatically saved** to the global credentials file so subsequent runs from any directory work without re-authenticating.

### Credentials storage

Credentials are stored in the OS config directory:

- **macOS:** `~/Library/Application Support/slacker/credentials.env`
- **Linux:** `~/.config/slacker/credentials.env`

You can also override with a `.env` file in the current directory (useful for project-specific workspaces) or by setting `SLACK_TOKEN` and `SLACK_COOKIE` environment variables directly.

### Getting your token and cookie manually

1. Open your Slack workspace in a browser
2. Open Developer Tools (F12) → Network tab
3. Look for any request to `api.slack.com`
4. Copy the `token` parameter (starts with `xoxc-`) and the `d` cookie value (starts with `xoxd-`)

## Configuration

### Timezone

Timestamps default to **UTC**. Set the `TZ` environment variable to display in your local timezone:

```bash
export TZ=America/Chicago
slacker "https://..."
```

Or set it in your shell profile (`~/.zshrc` or `~/.bashrc`) for persistence.

Valid values are any [IANA timezone name](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones) (e.g., `America/New_York`, `Europe/London`, `Asia/Tokyo`).

## User cache

Slacker caches workspace user data (display names and usernames) to avoid hitting the Slack API on every run. The cache is stored at:

- **macOS:** `~/Library/Caches/slacker/users.json`
- **Linux:** `~/.cache/slacker/users.json`

The cache expires after **24 hours** and is automatically refreshed on the next run. To force a refresh, delete the cache file.

## Project structure

```
slacker/
  main.go      — entry point, auth, CLI orchestration
  format.go    — text formatting, @mention replacement, timezone handling
  parse.go     — Slack URL parsing, timestamp extraction
  cache.go     — user cache (JSON file with 24h TTL)
```

## Key design decisions

- **stdout-only output:** All conversation text goes to stdout. Errors and status messages (auth saved, etc.) go to stderr. This makes slacker composable with pipes and redirects — ideal for use by AI agents or scripts.
- **Reimplemented internals:** slackdump's URL parser and text formatter live under `internal/`, which Go prevents external modules from importing. Slacker reimplements two small pieces: URL-to-timestamp parsing (~15 lines) and text formatting (~60 lines).
- **Silent by default:** slackdump's internal logging is suppressed via a discard logger to keep output clean and minimize token usage when used by AI agents.
- **Thread-aware formatting:** When dumping a thread URL, slackdump returns all messages flat (parent first, then replies). Slacker detects this via `conv.IsThread()` and applies reply indentation. For channel dumps, replies come nested in `ThreadReplies` and are handled recursively.

## Dependencies

- [slackdump v4](https://github.com/rusq/slackdump) — Slack API client and message dumper
- [godotenv](https://github.com/joho/godotenv) — `.env` file loading
