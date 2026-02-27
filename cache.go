package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rusq/slackdump/v4"
)

type userInfo struct {
	Name     string `json:"name"`
	Username string `json:"username"`
}

type userCache struct {
	FetchedAt time.Time            `json:"fetched_at"`
	Users     map[string]*userInfo `json:"users"`
}

const cacheTTL = 24 * time.Hour

// loadOrFetchUsers returns a userID->userInfo map, using a cached file if fresh enough.
func loadOrFetchUsers(ctx context.Context, sess *slackdump.Session) (map[string]*userInfo, error) {
	cachePath, err := cacheFilePath()
	if err != nil {
		return nil, err
	}

	if cached, err := loadCache(cachePath); err == nil {
		if time.Since(cached.FetchedAt) < cacheTTL {
			return cached.Users, nil
		}
	}

	users, err := sess.GetUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching users: %w", err)
	}

	userMap := make(map[string]*userInfo, len(users))
	for _, u := range users {
		name := u.RealName
		if name == "" {
			name = u.Name
		}
		userMap[u.ID] = &userInfo{Name: name, Username: u.Name}
	}

	// Save to cache (best-effort, ignore errors)
	_ = saveCache(cachePath, &userCache{
		FetchedAt: time.Now(),
		Users:     userMap,
	})

	return userMap, nil
}

func cacheFilePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("getting cache directory: %w", err)
	}
	return filepath.Join(dir, "slacker", "users.json"), nil
}

func loadCache(path string) (*userCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c userCache
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveCache(path string, c *userCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
