---
name: slacker
description: "Fetch and read Slack conversations using the slacker CLI. Use when: (1) the user says '/slacker', (2) the user shares a Slack URL (matching https://*.slack.com/archives/*), or (3) the user asks to read, fetch, or dump a Slack thread or channel. Slacker outputs human-readable conversation text to stdout."
---

# Slacker

CLI tool that dumps Slack conversations to stdout. Installed on `$PATH` via `go install`.

## Commands

```bash
# Dump a thread
slacker "<slack-url>"

# Dump a range of messages (same channel, two message URLs)
slacker "<start-url>" "<end-url>"

# JSON output for programmatic use
slacker --json "<slack-url>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON instead of plain text |
| `--no-interactive` | Fail instead of opening browser for auth |
| `--help`, `-h` | Show help message |
| `--version` | Show version |

## Output format (plain text, default)

```
> Alice Smith (@asmith) @ 2026-02-26 23:53:32 CST:
Hey @bjones, check out this thread

|   > Bob Jones (@bjones) @ 2026-02-26 23:54:10 CST:
|   Thanks @asmith!
```

- Headers: `> Full Name (@username) @ YYYY-MM-DD HH:MM:SS TZ:`
- Thread replies indented with `|   ` prefix
- `<@U123>` mentions resolved to `@username`
- Timestamps in UTC by default, or the timezone set in `TZ` env var

## Output format (JSON, `--json`)

```json
{
  "messages": [
    {
      "user": "Alice Smith",
      "username": "asmith",
      "timestamp": "2026-02-26T23:53:32-06:00",
      "text": "Hey @bjones, check out this thread",
      "replies": [...]
    }
  ]
}
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Usage error (bad arguments, invalid URL) |
| 3 | Authentication error |
| 4 | Network/API error |

## Authentication errors

Slacker requires Slack credentials. Use `--no-interactive` to prevent the tool from hanging on browser auth prompts.

If authentication fails (exit code 3), tell the user:

> Slacker needs to authenticate. Run this command in your terminal (outside of Claude Code), then try again:
>
> ```
> slacker "<the-url-they-gave-you>"
> ```
>
> This will open a browser for login and save credentials globally for future runs.

**Important:** Always use `--no-interactive` when invoking slacker from Claude Code to avoid hanging on browser auth. If it fails with exit code 3, prompt the user to authenticate manually.

## Tips

- Pipe to `pbcopy` for clipboard: `slacker "<url>" | pbcopy`
- Pipe to a file: `slacker "<url>" > thread.txt`
- Use `--json` with `jq` for structured extraction: `slacker --json "<url>" | jq '.messages[0].text'`
- The `(@username)` in output is the user's Slack handle — use it when drafting replies that @mention people
