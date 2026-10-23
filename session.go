package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sessionCache stores short-lived login tokens so successive CLI invocations do not
// trigger the server's login rate limiter. Passwords are never written to disk.
type sessionCache struct {
	Sessions map[string]sessionEntry `json:"sessions"`
}

type sessionEntry struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func sessionKey(url, username string) string {
	return strings.TrimRight(url, "/") + "|" + username
}

func sessionCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return "", fmt.Errorf("no user cache directory available")
	}
	return filepath.Join(dir, "fbcli", "sessions.json"), nil
}

func readSessionCache() sessionCache {
	cache := sessionCache{Sessions: map[string]sessionEntry{}}
	path, err := sessionCachePath()
	if err != nil {
		return cache
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	if err := json.Unmarshal(data, &cache); err != nil || cache.Sessions == nil {
		return sessionCache{Sessions: map[string]sessionEntry{}}
	}
	return cache
}

func writeSessionCache(cache sessionCache) {
	path, err := sessionCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return
	}
	_ = os.Chmod(path, 0o600)
}

// tokenExpiry extracts the "exp" claim from a JWT without validating the signature.
func tokenExpiry(token string) time.Time {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}
	}
	var claims struct {
		ExpiresAt int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.ExpiresAt == 0 {
		return time.Time{}
	}
	return time.Unix(claims.ExpiresAt, 0)
}

// LoadSession restores a cached token if it is still comfortably valid.
func (c *Client) LoadSession() bool {
	entry, ok := readSessionCache().Sessions[sessionKey(c.Config.URL, c.Config.Username)]
	if !ok || entry.Token == "" {
		return false
	}
	if time.Now().After(entry.ExpiresAt.Add(-2 * time.Minute)) {
		return false
	}
	c.Token = entry.Token
	return true
}

// SaveSession caches the current token until shortly before it expires.
func (c *Client) SaveSession() {
	if c.Token == "" {
		return
	}
	expires := tokenExpiry(c.Token)
	if expires.IsZero() {
		expires = time.Now().Add(30 * time.Minute)
	}
	if time.Now().After(expires.Add(-2 * time.Minute)) {
		return
	}

	cache := readSessionCache()
	cache.Sessions[sessionKey(c.Config.URL, c.Config.Username)] = sessionEntry{
		Token:     c.Token,
		ExpiresAt: expires,
	}
	writeSessionCache(cache)
}
