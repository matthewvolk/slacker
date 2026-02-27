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
```

## Output format

```
> Alice Smith (@asmith) @ 26/02/2026 11:53:32 PM CST:
Hey @bjones, check out this thread

|   > Bob Jones (@bjones) @ 26/02/2026 11:54:10 PM CST:
|   Thanks @asmith!
```

- Headers: `> Full Name (@username) @ DD/MM/YYYY H:MM:SS AM/PM TZ:`
- Thread replies indented with `|   ` prefix
- `<@U123>` mentions resolved to `@username`
- Timestamps in UTC by default, or the timezone set in `TZ` env var

## Authentication errors

Slacker requires Slack credentials. If the command hangs or prompts for interactive login (you may see a charmbracelet/huh TUI prompt about login method), **stop immediately** — do not attempt to authenticate.

Tell the user:

> Slacker needs to re-authenticate. Run this command in your terminal (outside of Claude Code), then try again:
>
> ```
> slacker "<the-url-they-gave-you>"
> ```
>
> This will open a browser for login and save credentials globally for future runs.

## Tips

- Pipe to `pbcopy` for clipboard: `slacker "<url>" | pbcopy`
- Pipe to a file: `slacker "<url>" > thread.txt`
- The `(@username)` in output is the user's Slack handle — use it when drafting replies that @mention people
