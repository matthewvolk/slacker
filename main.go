package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/rusq/slackdump/v4"
	"github.com/rusq/slackdump/v4/auth"
	"github.com/rusq/slackdump/v4/types"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// credentialsFilePath returns the path to the global credentials file.
// macOS: ~/Library/Application Support/slacker/credentials.env
// Linux: ~/.config/slacker/credentials.env
func credentialsFilePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "slacker", "credentials.env"), nil
}

func run() error {
	// Load credentials: cwd .env first (project override), then global config (fallback).
	// godotenv won't overwrite vars already set, so cwd .env takes priority.
	_ = godotenv.Load()
	if credPath, err := credentialsFilePath(); err == nil {
		_ = godotenv.Load(credPath)
	}

	args := os.Args[1:]

	// Handle subcommands before URL-based logic
	if len(args) >= 1 && args[0] == "tz" {
		return handleTZ(args[1:])
	}
	if len(args) >= 1 && args[0] == "purge" {
		return handlePurge()
	}

	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("usage: slacker <thread-url> [end-url]\n       slacker tz [timezone]\n       slacker purge")
	}

	ctx := context.Background()

	prov, err := authenticate(ctx, args[0])
	if err != nil {
		return fmt.Errorf("authentication: %w", err)
	}

	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess, err := slackdump.New(ctx, prov, slackdump.WithLogger(nopLogger))
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	userMap, err := loadOrFetchUsers(ctx, sess)
	if err != nil {
		return fmt.Errorf("loading users: %w", err)
	}

	var conv *types.Conversation

	switch len(args) {
	case 1:
		conv, err = sess.DumpAll(ctx, args[0])
	case 2:
		_, oldest, parseErr := parseSlackURL(args[0])
		if parseErr != nil {
			return fmt.Errorf("parsing start URL: %w", parseErr)
		}
		if oldest.IsZero() {
			return fmt.Errorf("start URL must include a message timestamp")
		}
		_, latest, parseErr := parseSlackURL(args[1])
		if parseErr != nil {
			return fmt.Errorf("parsing end URL: %w", parseErr)
		}
		if latest.IsZero() {
			return fmt.Errorf("end URL must include a message timestamp")
		}
		conv, err = sess.Dump(ctx, args[0], oldest, latest)
	}
	if err != nil {
		return fmt.Errorf("dumping messages: %w", err)
	}

	formatConversation(os.Stdout, conv, userMap)
	return nil
}

func handleTZ(args []string) error {
	credPath, err := credentialsFilePath()
	if err != nil {
		return fmt.Errorf("config directory: %w", err)
	}

	// Read existing config (ignore error if file doesn't exist yet)
	envMap, _ := godotenv.Read(credPath)
	if envMap == nil {
		envMap = make(map[string]string)
	}

	// No arg: show current setting
	if len(args) == 0 {
		if tz, ok := envMap["TZ"]; ok {
			fmt.Println(tz)
		} else {
			fmt.Println("UTC (default)")
		}
		return nil
	}

	// Validate the timezone
	tz := args[0]
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("invalid timezone %q — use an IANA name like America/New_York or America/Chicago", tz)
	}

	envMap["TZ"] = tz

	if err := os.MkdirAll(filepath.Dir(credPath), 0o755); err != nil {
		return err
	}
	if err := godotenv.Write(envMap, credPath); err != nil {
		return err
	}
	_ = os.Chmod(credPath, 0o600)

	fmt.Fprintf(os.Stderr, "Timezone set to %s\n", tz)
	return nil
}

func handlePurge() error {
	dir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("cache directory: %w", err)
	}
	cacheDir := filepath.Join(dir, "slacker")

	if err := os.RemoveAll(cacheDir); err != nil {
		return fmt.Errorf("removing cache: %w", err)
	}

	fmt.Fprintln(os.Stderr, "Cache purged.")
	return nil
}

func authenticate(ctx context.Context, slackURL string) (auth.Provider, error) {
	token := os.Getenv("SLACK_TOKEN")
	if token != "" {
		cookie := os.Getenv("SLACK_COOKIE")
		prov, err := auth.NewValueAuth(token, cookie)
		if err != nil {
			return nil, err
		}
		return prov, nil
	}

	workspace, err := extractWorkspace(slackURL)
	if err != nil {
		return nil, fmt.Errorf("extracting workspace for browser auth: %w", err)
	}

	prov, err := auth.NewRODAuth(ctx, auth.BrowserWithWorkspace(workspace))
	if err != nil {
		return nil, err
	}

	credPath, pathErr := credentialsFilePath()
	if pathErr != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine config directory: %v\n", pathErr)
	} else if err := saveCredentials(credPath, prov); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save credentials: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Credentials saved to %s\n", credPath)
	}

	return prov, nil
}

// saveCredentials writes SLACK_TOKEN and SLACK_COOKIE to the credentials file,
// preserving any other settings (like TZ) already stored there.
func saveCredentials(path string, prov auth.Provider) error {
	token := prov.SlackToken()
	var cookie string
	for _, c := range prov.Cookies() {
		if c.Name == "d" {
			cookie = c.Value
			break
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Read existing config to preserve non-credential settings (e.g., TZ)
	envMap, _ := godotenv.Read(path)
	if envMap == nil {
		envMap = make(map[string]string)
	}

	envMap["SLACK_TOKEN"] = token
	if cookie != "" {
		envMap["SLACK_COOKIE"] = cookie
	}

	if err := godotenv.Write(envMap, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
