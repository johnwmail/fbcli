package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config holds the connection settings read from the environment or interactive prompts.
type Config struct {
	URL      string
	Username string
	Password string
	TOTP     string
	Source   string
}

// Client talks to a FileBrowser Quantum (v2.x) server.
type Client struct {
	Config Config
	Token  string
	Source string

	http *http.Client
}

// APIError is returned for non-successful API responses so callers can inspect the status code.
type APIError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, http.StatusText(e.StatusCode))
	}
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func isNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func isUnauthorized(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusUnauthorized
}

func NewClient(cfg Config) *Client {
	return &Client{
		Config: cfg,
		http: &http.Client{
			// Transfers can be long-running; individual requests are bounded by the
			// server and the OS connection timeouts instead of an arbitrary client timeout.
			Timeout: 0,
		},
	}
}

// resourceURL builds an absolute API URL and attaches the selected source.
func (c *Client) resourceURL(endpoint string, query url.Values) string {
	if query == nil {
		query = url.Values{}
	}
	if c.Source != "" && strings.HasPrefix(endpoint, "/api/resources") {
		query.Set("source", c.Source)
	}
	u := strings.TrimRight(c.Config.URL, "/") + endpoint
	if encoded := query.Encode(); encoded != "" {
		u += "?" + encoded
	}
	return u
}

func (c *Client) do(method, endpoint string, query url.Values, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, c.resourceURL(endpoint, query), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	if c.Token != "" {
		// Bearer auth is preferred by v2 servers; the cookie keeps compatibility with
		// servers that still extract the token from the session cookie.
		req.Header.Set("Authorization", "Bearer "+c.Token)
		req.AddCookie(&http.Cookie{Name: authCookieName, Value: c.Token})
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return c.http.Do(req)
}

const authCookieName = "filebrowser_quantum_jwt"

// Login authenticates against /api/auth/login. v2 expects the username in the query
// string and the URL-encoded password in the X-Password header.
func (c *Client) Login() error {
	err := c.loginOnce()
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests {
		// The server's credential limiter is per IP and per username. A short wait is
		// usually enough; longer waits are returned so scripts do not hang.
		if apiErr.RetryAfter > 15*time.Second {
			return err
		}
		wait := 5 * time.Second
		if apiErr.RetryAfter > 0 {
			wait = apiErr.RetryAfter
		}
		fmt.Fprintf(os.Stderr, "Login rate limited; retrying in %s...\n", wait)
		time.Sleep(wait)
		return c.loginOnce()
	}
	return err
}

func (c *Client) loginOnce() error {
	query := url.Values{}
	query.Set("username", c.Config.Username)
	headers := map[string]string{
		"X-Password": url.QueryEscape(c.Config.Password),
	}
	if c.Config.TOTP != "" {
		headers["X-Secret"] = c.Config.TOTP
	}

	resp, err := c.do(http.MethodPost, "/api/auth/login", query, nil, headers)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", c.Config.URL, err)
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}

	token, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read login response: %w", err)
	}
	c.Token = strings.TrimSpace(string(token))
	if c.Token == "" {
		return errors.New("login succeeded but the server returned an empty token")
	}
	return nil
}

type sourceInfo struct {
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
}

// ResolveSource picks the source used for all resource operations. FileBrowser Quantum
// supports multiple "sources"; the CLI needs exactly one per command.
func (c *Client) ResolveSource() error {
	resp, err := c.do(http.MethodGet, "/api/settings/sources", nil, nil, nil)
	if err != nil {
		return fmt.Errorf("list sources: %w", err)
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}

	var sources map[string]sourceInfo
	if err := json.NewDecoder(resp.Body).Decode(&sources); err != nil {
		return fmt.Errorf("decode sources: %w", err)
	}

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return errors.New("the account has no accessible sources; ask an administrator to grant access")
	}

	if c.Config.Source != "" {
		if _, ok := sources[c.Config.Source]; !ok {
			return fmt.Errorf("source %q not found (available: %s)", c.Config.Source, strings.Join(names, ", "))
		}
		c.Source = c.Config.Source
		return nil
	}

	c.Source = names[0]
	if len(names) > 1 {
		fmt.Fprintf(os.Stderr,
			"Note: multiple sources are available (%s); using %q. Set FILEBROWSER_SOURCE to choose another.\n",
			strings.Join(names, ", "), c.Source)
	}
	return nil
}

// Resource describes a file or folder returned by /api/resources.
type Resource struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Type     string    `json:"type"`
	IsDir    bool      `json:"-"`

	Files   []Resource `json:"files,omitempty"`
	Folders []Resource `json:"folders,omitempty"`
}

// getResource returns metadata for one remote path (directories include their children).
func (c *Client) getResource(remotePath string) (*Resource, error) {
	query := url.Values{}
	query.Set("path", normalizeRemotePath(remotePath))
	query.Set("skipExtendedAttrs", "true")

	resp, err := c.do(http.MethodGet, "/api/resources", query, nil, nil)
	if err != nil {
		return nil, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}

	var item Resource
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decode resource %q: %w", remotePath, err)
	}
	item.IsDir = item.Type == "directory"
	return &item, nil
}

// listRemote returns the direct children of a remote directory.
func (c *Client) listRemote(remotePath string) ([]Resource, error) {
	dir, err := c.getResource(remotePath)
	if err != nil {
		return nil, err
	}
	if !dir.IsDir {
		return nil, fmt.Errorf("%s is not a directory", normalizeRemotePath(remotePath))
	}

	// A map guards against duplicate/ghost entries while preserving directory semantics.
	items := make(map[string]Resource, len(dir.Files)+len(dir.Folders))
	for _, folder := range dir.Folders {
		folder.IsDir = true
		items[folder.Name] = folder
	}
	for _, file := range dir.Files {
		if _, exists := items[file.Name]; exists {
			continue
		}
		file.IsDir = file.Type == "directory"
		items[file.Name] = file
	}

	result := make([]Resource, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result, nil
}

// getExactFileSize asks the download endpoint for the real byte length. The index may
// report disk usage (for example 4096 bytes for a 6-byte file), which is unsuitable
// for sync comparisons.
func (c *Client) getExactFileSize(remotePath string) (int64, error) {
	query := url.Values{}
	query.Set("file", normalizeRemotePath(remotePath))

	resp, err := c.do(http.MethodHead, "/api/resources/download", query, nil, nil)
	if err != nil {
		return 0, err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return 0, responseError(resp)
	}
	if resp.ContentLength < 0 {
		return 0, errors.New("server did not report a content length")
	}
	return resp.ContentLength, nil
}

// getChecksum returns the server-side checksum for a file.
func (c *Client) getChecksum(remotePath, algorithm string) (string, error) {
	query := url.Values{}
	query.Set("path", normalizeRemotePath(remotePath))
	query.Set("checksum", algorithm)

	resp, err := c.do(http.MethodGet, "/api/resources", query, nil, nil)
	if err != nil {
		return "", err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return "", responseError(resp)
	}

	var data struct {
		Checksums map[string]string `json:"checksums"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("decode checksum response: %w", err)
	}
	value := data.Checksums[algorithm]
	if value == "" {
		return "", fmt.Errorf("server did not return a %s checksum", algorithm)
	}
	return strings.ToLower(strings.TrimSpace(value)), nil
}

func responseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	message := strings.TrimSpace(string(body))

	var envelope struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Message != "" {
		message = envelope.Message
	}
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	apiErr := &APIError{StatusCode: resp.StatusCode, Message: message}
	if seconds, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); err == nil && seconds > 0 {
		apiErr.RetryAfter = time.Duration(seconds) * time.Second
	}
	return apiErr
}

func closeResponse(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

// normalizeRemotePath makes a user-supplied path safe for the API query parameters.
func normalizeRemotePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return pathClean(p)
}

// pathClean is separated out so it is easy to test and keeps remote paths slash based.
func pathClean(p string) string {
	cleaned := strings.TrimRight(p, "/")
	if cleaned == "" {
		return "/"
	}
	// Collapse duplicate slashes and dot segments without importing filepath (which
	// would use backslashes on Windows).
	parts := strings.Split(cleaned, "/")
	stack := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		default:
			stack = append(stack, part)
		}
	}
	return "/" + strings.Join(stack, "/")
}
