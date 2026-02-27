package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// parseSlackURL extracts the channel ID and message timestamp from a Slack archive URL.
// URL format: https://workspace.slack.com/archives/CHANID/pTIMESTAMP
func parseSlackURL(rawURL string) (channelID string, ts time.Time, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid URL: %w", err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "archives" {
		return "", time.Time{}, fmt.Errorf("not a Slack archive URL: %s", rawURL)
	}

	channelID = parts[1]

	if len(parts) >= 3 {
		ts, err = parseThreadID(parts[2])
		if err != nil {
			return "", time.Time{}, err
		}
	}

	return channelID, ts, nil
}

// parseThreadID converts a Slack thread ID (e.g., "p1577694990000400") to time.Time.
// Format: 'p' followed by 16 digits (10 seconds + 6 microseconds).
func parseThreadID(threadID string) (time.Time, error) {
	if !strings.HasPrefix(threadID, "p") || len(threadID) != 17 {
		return time.Time{}, fmt.Errorf("invalid thread ID: %s", threadID)
	}

	usec, err := strconv.ParseInt(threadID[1:], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid thread ID: %s", threadID)
	}

	return time.UnixMicro(usec).UTC(), nil
}

// extractWorkspace extracts the workspace name from a Slack URL hostname.
// e.g., "https://mycompany.slack.com/..." -> "mycompany"
func extractWorkspace(slackURL string) (string, error) {
	u, err := url.Parse(slackURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := u.Hostname()
	if !strings.HasSuffix(host, ".slack.com") {
		return "", fmt.Errorf("not a Slack URL: %s", slackURL)
	}

	workspace := strings.TrimSuffix(host, ".slack.com")
	if workspace == "" {
		return "", fmt.Errorf("no workspace in URL: %s", slackURL)
	}

	return workspace, nil
}
