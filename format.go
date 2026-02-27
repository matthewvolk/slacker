package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rusq/slackdump/v4/types"
)

const timeFormat = "2006-01-02 15:04:05 MST"

// loadTimezone returns the location from the TZ env var, or UTC if unset/invalid.
func loadTimezone() *time.Location {
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return time.UTC
}

var mentionRe = regexp.MustCompile(`<@([UW][A-Z0-9]+)>`)

// formatConversation writes a conversation to w in a human-readable text format.
// For thread dumps, slackdump returns all messages flat (parent first, then replies),
// so we indent everything after the first message. For channel dumps, replies are
// nested in ThreadReplies and handled recursively by formatMessages.
func formatConversation(w io.Writer, conv *types.Conversation, userMap map[string]*userInfo) {
	loc := loadTimezone()
	if conv.IsThread() && len(conv.Messages) > 0 {
		formatMessages(w, conv.Messages[:1], userMap, "", loc)
		formatMessages(w, conv.Messages[1:], userMap, "|   ", loc)
	} else {
		formatMessages(w, conv.Messages, userMap, "", loc)
	}
}

// formatMessages writes messages with the given prefix (used for thread indentation).
func formatMessages(w io.Writer, messages []types.Message, userMap map[string]*userInfo, prefix string, loc *time.Location) {
	for _, msg := range messages {
		t, _ := msg.Datetime()
		t = t.In(loc)
		name, username := displayName(msg.User, userMap)
		text := replaceMentions(html.UnescapeString(msg.Text), userMap)

		fmt.Fprintf(w, "\n%s> %s (@%s) @ %s:\n", prefix, name, username, t.Format(timeFormat))
		for _, line := range strings.Split(text, "\n") {
			fmt.Fprintf(w, "%s%s\n", prefix, line)
		}

		if len(msg.ThreadReplies) > 0 {
			formatMessages(w, msg.ThreadReplies, userMap, prefix+"|   ", loc)
		}
	}
}

// jsonMessage represents a single message in JSON output.
type jsonMessage struct {
	User      string        `json:"user"`
	Username  string        `json:"username"`
	Timestamp string        `json:"timestamp"`
	Text      string        `json:"text"`
	Replies   []jsonMessage `json:"replies,omitempty"`
}

// formatConversationJSON writes a conversation to w as JSON.
func formatConversationJSON(w io.Writer, conv *types.Conversation, userMap map[string]*userInfo) {
	loc := loadTimezone()

	var messages []jsonMessage
	if conv.IsThread() && len(conv.Messages) > 0 {
		parent := toJSONMessage(conv.Messages[0], userMap, loc)
		parent.Replies = toJSONMessages(conv.Messages[1:], userMap, loc)
		messages = append(messages, parent)
	} else {
		messages = toJSONMessages(conv.Messages, userMap, loc)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		Messages []jsonMessage `json:"messages"`
	}{Messages: messages})
}

func toJSONMessages(messages []types.Message, userMap map[string]*userInfo, loc *time.Location) []jsonMessage {
	out := make([]jsonMessage, 0, len(messages))
	for _, msg := range messages {
		out = append(out, toJSONMessage(msg, userMap, loc))
	}
	return out
}

func toJSONMessage(msg types.Message, userMap map[string]*userInfo, loc *time.Location) jsonMessage {
	t, _ := msg.Datetime()
	t = t.In(loc)
	name, username := displayName(msg.User, userMap)
	text := replaceMentions(html.UnescapeString(msg.Text), userMap)

	jm := jsonMessage{
		User:      name,
		Username:  username,
		Timestamp: t.Format(time.RFC3339),
		Text:      text,
	}

	if len(msg.ThreadReplies) > 0 {
		jm.Replies = toJSONMessages(msg.ThreadReplies, userMap, loc)
	}

	return jm
}

// displayName returns the display name and username for a user ID.
func displayName(userID string, userMap map[string]*userInfo) (string, string) {
	if info, ok := userMap[userID]; ok {
		return info.Name, info.Username
	}
	return userID, userID
}

// replaceMentions converts Slack mention markup (<@U123ABC>) to @username.
func replaceMentions(text string, userMap map[string]*userInfo) string {
	return mentionRe.ReplaceAllStringFunc(text, func(match string) string {
		matches := mentionRe.FindStringSubmatch(match)
		if len(matches) == 2 {
			if info, ok := userMap[matches[1]]; ok {
				return "@" + info.Username
			}
		}
		return match
	})
}
