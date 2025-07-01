package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	"github.com/rusq/slackdump/v3"
	"github.com/rusq/slackdump/v3/auth"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <slack-thread-or-channel-URL>")
		os.Exit(1)
	}

	err := godotenv.Load()
	if err != nil {
		log.Println("️[WARNING] No .env file found, using shell environment")
	}

	url := os.Args[1]
	xoxc := os.Getenv("SLACK_XOXC_TOKEN")
	xoxd := os.Getenv("SLACK_XOXD_COOKIE")

	if xoxc == "" || xoxd == "" {
		log.Fatal("️[ERROR] SLACK_XOXC_TOKEN and SLACK_XOXD_COOKIE must be set")
	}

	provider, err := auth.NewValueAuth(xoxc, xoxd)
	if err != nil {
		log.Fatalf("️[ERROR] auth error: %v", err)
	}

	ctx := context.Background()
	sd, err := slackdump.New(ctx, provider)
	if err != nil {
		log.Fatalf("️[ERROR] session error: %v", err)
	}

	fmt.Println("️[INFO] Dumping messages...")

	oldest := time.Time{}
	latest := time.Now().Add(1 * time.Hour)

	conv, err := sd.Dump(ctx, url, oldest, latest)
	if err != nil {
		log.Fatalf("️[ERROR] dump error: %v", err)
	}

	users, err := sd.GetUsers(ctx)
	if err != nil {
		log.Fatalf("️[ERROR] failed to get users: %v", err)
	}

	userMap := make(map[string]string)
	for _, user := range users {
		name := user.RealName
		if name == "" {
			name = user.Name
		}
		userMap[user.ID] = name
	}

	fmt.Printf("\n[SUCCESS] Exported %d messages:\n\n", len(conv.Messages))
	for _, msg := range conv.Messages {
		user := userMap[msg.User]
		if user == "" {
			user = "(unknown user)"
		}
		t := parseSlackTimestamp(msg.Timestamp)
		fmt.Printf("[%s] %s:\n%s\n\n", t.Format("Jan 02 15:04"), user, replaceMentions(msg.Text, userMap))
	}
}

func parseSlackTimestamp(ts string) time.Time {
	float, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return time.Time{}
	}
	secs := int64(float)
	nanos := int64((float - float64(secs)) * 1e9)
	return time.Unix(secs, nanos)
}

func replaceMentions(text string, userMap map[string]string) string {
	re := regexp.MustCompile(`<@([UW][A-Z0-9]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) == 2 {
			if name, ok := userMap[matches[1]]; ok {
				return "@" + name
			}
		}
		return match
	})
}
