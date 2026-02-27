package main

import (
	"context"
	"errors"
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

var version = "dev"

// Exit codes
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
	exitAuth  = 3
	exitAPI   = 4
)

// exitError is a sentinel error that carries a specific exit code.
type codeError struct {
	code int
	err  error
}

func (e *codeError) Error() string { return e.err.Error() }
func (e *codeError) Unwrap() error { return e.err }

func usageErr(err error) error { return &codeError{code: exitUsage, err: err} }
func authErr(err error) error  { return &codeError{code: exitAuth, err: err} }
func apiErr(err error) error   { return &codeError{code: exitAPI, err: err} }

type flags struct {
	help          bool
	version       bool
	json          bool
	noInteractive bool
}

func parseFlags(args []string) ([]string, flags) {
	var f flags
	var positional []string
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "help":
			f.help = true
		case "--version":
			f.version = true
		case "--json":
			f.json = true
		case "--no-interactive":
			f.noInteractive = true
		default:
			positional = append(positional, arg)
		}
	}
	return positional, f
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		code := exitError
		var ce *codeError
		if errors.As(err, &ce) {
			code = ce.code
		}
		os.Exit(code)
	}
}

func printUsage() {
	fmt.Println(`Usage: slacker [flags] <thread-url> [end-url]
       slacker tz [timezone]
       slacker purge

Dump Slack conversations as readable text.

Commands:
  tz [timezone]   Show or set the display timezone (IANA name)
  purge           Clear cached user data

Flags:
  --help, -h        Show this help message
  --version         Show version
  --json            Output as JSON instead of plain text
  --no-interactive  Fail instead of opening browser for auth`)
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

	args, fl := parseFlags(os.Args[1:])

	if fl.help {
		printUsage()
		return nil
	}
	if fl.version {
		fmt.Println("slacker " + version)
		return nil
	}

	// Handle subcommands before URL-based logic
	if len(args) >= 1 && args[0] == "tz" {
		return handleTZ(args[1:])
	}
	if len(args) >= 1 && args[0] == "purge" {
		return handlePurge()
	}

	if len(args) < 1 || len(args) > 2 {
		return usageErr(fmt.Errorf("usage: slacker <thread-url> [end-url]\n       slacker tz [timezone]\n       slacker purge\n\nRun 'slacker --help' for more information."))
	}

	ctx := context.Background()

	prov, err := authenticate(ctx, args[0], fl.noInteractive)
	if err != nil {
		return authErr(fmt.Errorf("authentication: %w", err))
	}

	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sess, err := slackdump.New(ctx, prov, slackdump.WithLogger(nopLogger))
	if err != nil {
		return apiErr(fmt.Errorf("creating session: %w", err))
	}

	userMap, err := loadOrFetchUsers(ctx, sess)
	if err != nil {
		return apiErr(fmt.Errorf("loading users: %w", err))
	}

	var conv *types.Conversation

	switch len(args) {
	case 1:
		conv, err = sess.DumpAll(ctx, args[0])
	case 2:
		_, oldest, parseErr := parseSlackURL(args[0])
		if parseErr != nil {
			return usageErr(fmt.Errorf("parsing start URL: %w", parseErr))
		}
		if oldest.IsZero() {
			return usageErr(fmt.Errorf("start URL must include a message timestamp"))
		}
		_, latest, parseErr := parseSlackURL(args[1])
		if parseErr != nil {
			return usageErr(fmt.Errorf("parsing end URL: %w", parseErr))
		}
		if latest.IsZero() {
			return usageErr(fmt.Errorf("end URL must include a message timestamp"))
		}
		conv, err = sess.Dump(ctx, args[0], oldest, latest)
	}
	if err != nil {
		return apiErr(fmt.Errorf("dumping messages: %w", err))
	}

	if fl.json {
		formatConversationJSON(os.Stdout, conv, userMap)
	} else {
		formatConversation(os.Stdout, conv, userMap)
	}
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

func authenticate(ctx context.Context, slackURL string, noInteractive bool) (auth.Provider, error) {
	token := os.Getenv("SLACK_TOKEN")
	if token != "" {
		cookie := os.Getenv("SLACK_COOKIE")
		prov, err := auth.NewValueAuth(token, cookie)
		if err != nil {
			return nil, err
		}
		return prov, nil
	}

	if noInteractive {
		return nil, fmt.Errorf("no credentials found (set SLACK_TOKEN or run without --no-interactive to log in via browser)")
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
