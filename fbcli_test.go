package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	options, args, err := parseOptions([]string{"-i", `.*\.log$`, "-l", "-S", "media", "/docs", "-s"})
	if err != nil {
		t.Fatalf("parseOptions returned an error: %v", err)
	}
	if options.ignore != `.*\.log$` {
		t.Errorf("ignore = %q", options.ignore)
	}
	if options.source != "media" {
		t.Errorf("source = %q", options.source)
	}
	if !options.long || !options.script {
		t.Errorf("expected long and script flags, got %+v", options)
	}
	if len(args) != 1 || args[0] != "/docs" {
		t.Errorf("args = %#v", args)
	}
}

func TestParseOptionsSeparator(t *testing.T) {
	options, args, err := parseOptions([]string{"--", "-not-a-flag", "--also-not"})
	if err != nil {
		t.Fatalf("parseOptions returned an error: %v", err)
	}
	if len(args) != 2 || args[0] != "-not-a-flag" || args[1] != "--also-not" {
		t.Errorf("args = %#v", args)
	}
	if options.ignore != "" {
		t.Errorf("ignore = %q", options.ignore)
	}
}

func TestParseOptionsRejectsUnknownFlag(t *testing.T) {
	if _, _, err := parseOptions([]string{"--nope"}); err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
}

func TestParseOptionsEqualsForms(t *testing.T) {
	options, args, err := parseOptions([]string{"--ignore=^tmp", "--source=media", "--zip", "--script", "--long"})
	if err != nil {
		t.Fatalf("parseOptions returned an error: %v", err)
	}
	if options.ignore != "^tmp" || options.source != "media" {
		t.Errorf("unexpected options: %+v", options)
	}
	if !options.zip || !options.script || !options.long {
		t.Errorf("expected zip/script/long flags: %+v", options)
	}
	if len(args) != 0 {
		t.Errorf("args = %#v", args)
	}
}

func TestParseOptionsHelp(t *testing.T) {
	if _, _, err := parseOptions([]string{"--help"}); !errors.Is(err, errHelpRequested) {
		t.Errorf("expected errHelpRequested, got %v", err)
	}
}

func TestNormalizeRemotePathSpecialCharacters(t *testing.T) {
	cases := map[string]string{
		"/docs/a b.txt":          "/docs/a b.txt",
		"/docs/檔案 #%&?.txt":      "/docs/檔案 #%&?.txt",
		"/docs//sub/../a.txt":    "/docs/a.txt",
		"/docs/../../etc/passwd": "/etc/passwd",
	}
	for input, expected := range cases {
		if actual := normalizeRemotePath(input); actual != expected {
			t.Errorf("normalizeRemotePath(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestNormalizeRemotePath(t *testing.T) {
	cases := map[string]string{
		"":              "/",
		"/":             "/",
		"docs":          "/docs",
		"/docs/":        "/docs",
		"//docs//a.txt": "/docs/a.txt",
		"/docs/../a":    "/a",
		"/a/./b/":       "/a/b",
	}
	for input, expected := range cases {
		if actual := normalizeRemotePath(input); actual != expected {
			t.Errorf("normalizeRemotePath(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestResolveZipTarget(t *testing.T) {
	tempDir := t.TempDir()
	if got := resolveZipTarget("/remote/docs", ""); got != "docs.zip" {
		t.Errorf("default target = %q", got)
	}
	if got := resolveZipTarget("/remote/docs", filepath.Join(tempDir, "out")); got != filepath.Join(tempDir, "out.zip") {
		t.Errorf("explicit target = %q", got)
	}
	if got := resolveZipTarget("/remote/docs", filepath.Join(tempDir, "out.zip")); got != filepath.Join(tempDir, "out.zip") {
		t.Errorf("existing suffix target = %q", got)
	}
	if got := resolveZipTarget("/remote/docs", filepath.Join(tempDir, "out.ZIP")); got != filepath.Join(tempDir, "out.zip") {
		t.Errorf("uppercase suffix target = %q", got)
	}

	// A directory with the zip name must not be clobbered; use a numeric suffix.
	conflict := filepath.Join(tempDir, "docs.zip")
	if err := os.Mkdir(conflict, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveZipTarget("/remote/docs", tempDir); got != filepath.Join(tempDir, "docs-1.zip") {
		t.Errorf("conflict target = %q", got)
	}
}

func TestRedactPassword(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"a":        "*",
		"ab":       "**",
		"secret":   "s****t",
		"password": "p******d",
	}
	for input, expected := range cases {
		if actual := redactPassword(input); actual != expected {
			t.Errorf("redactPassword(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestTokenExpiry(t *testing.T) {
	// {"exp": 1700000000}
	token := "header.eyJleHAiOjE3MDAwMDAwMDB9.signature"
	expires := tokenExpiry(token)
	if !expires.Equal(time.Unix(1700000000, 0)) {
		t.Errorf("tokenExpiry = %v", expires)
	}
	if !tokenExpiry("not-a-jwt").IsZero() {
		t.Error("expected malformed token to have no expiry")
	}
}

func TestSortResources(t *testing.T) {
	older := time.Unix(1000, 0)
	newer := time.Unix(2000, 0)
	items := []Resource{
		{Name: "b", Modified: older},
		{Name: "a", Modified: older},
		{Name: "c", Modified: newer},
	}
	sortResources(items)
	if items[0].Name != "c" || items[1].Name != "a" || items[2].Name != "b" {
		t.Errorf("unexpected order: %#v", items)
	}
}

func TestDisplayName(t *testing.T) {
	dir := Resource{Name: "docs", IsDir: true}
	if got := displayName(dir, false); got != "docs/" {
		t.Errorf("displayName(dir) = %q", got)
	}
	if got := displayName(dir, true); !strings.Contains(got, colorBlue) || !strings.HasSuffix(got, "/"+colorReset) {
		t.Errorf("colored displayName(dir) = %q", got)
	}
	file := Resource{Name: "a.txt"}
	if got := displayName(file, true); got != "a.txt" {
		t.Errorf("displayName(file) = %q", got)
	}
}

func TestStripANSI(t *testing.T) {
	input := colorBlue + "folder/" + colorReset
	if got := stripANSI(input); got != "folder/" {
		t.Errorf("stripANSI = %q", got)
	}
}

func TestResolveLocalFileTarget(t *testing.T) {
	tempDir := t.TempDir()
	if got := resolveLocalFileTarget("/remote/a.txt", ""); got != "a.txt" {
		t.Errorf("default target = %q", got)
	}
	if got := resolveLocalFileTarget("/remote/a.txt", tempDir); got != filepath.Join(tempDir, "a.txt") {
		t.Errorf("existing directory target = %q", got)
	}
	if got := resolveLocalFileTarget("/remote/a.txt", filepath.Join(tempDir, "renamed.txt")); got != filepath.Join(tempDir, "renamed.txt") {
		t.Errorf("explicit target = %q", got)
	}
}

func TestWriteResponseToFile(t *testing.T) {
	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "nested", "file.txt")
	if err := writeResponseToFile(bytes.NewBufferString("content"), target); err != nil {
		t.Fatalf("writeResponseToFile returned an error: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Errorf("content = %q", data)
	}
}

func TestHashReader(t *testing.T) {
	// sha256("abc")
	const expected = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	actual, err := hashReader(strings.NewReader("abc"))
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Errorf("hashReader = %q, want %q", actual, expected)
	}
}

func TestAPIError(t *testing.T) {
	err := &APIError{StatusCode: 404, Message: "not found"}
	if got := err.Error(); got != "HTTP 404: not found" {
		t.Errorf("APIError.Error = %q", got)
	}
	err = &APIError{StatusCode: 403}
	if got := err.Error(); got != "HTTP 403: Forbidden" {
		t.Errorf("APIError.Error = %q", got)
	}
	if !isNotFound(&APIError{StatusCode: 404}) {
		t.Error("expected isNotFound to be true")
	}
	if isUnauthorized(&APIError{StatusCode: 403}) {
		t.Error("expected isUnauthorized to be false")
	}
}

func TestSessionKey(t *testing.T) {
	if got := sessionKey("http://example.com/", "alice"); got != "http://example.com|alice" {
		t.Errorf("sessionKey = %q", got)
	}
}

func TestResponseErrorRetryAfter(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"12"}},
		Body:       io.NopCloser(strings.NewReader(`{"message":"slow down"}`)),
	}
	err := responseError(resp)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Message != "slow down" {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
	if apiErr.RetryAfter != 12*time.Second {
		t.Errorf("RetryAfter = %s", apiErr.RetryAfter)
	}
}

func TestLoginUsesV2Headers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/auth/login" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("username"); got != "alice" {
			t.Errorf("username = %q", got)
		}
		password, err := url.QueryUnescape(r.Header.Get("X-Password"))
		if err != nil || password != "p@ss word/with+chars" {
			t.Errorf("password = %q, err = %v", password, err)
		}
		if got := r.Header.Get("X-Secret"); got != "123456" {
			t.Errorf("X-Secret = %q", got)
		}
		_, _ = w.Write([]byte("test-token"))
	}))
	defer server.Close()

	client := NewClient(Config{
		URL:      server.URL,
		Username: "alice",
		Password: "p@ss word/with+chars",
		TOTP:     "123456",
	})
	if err := client.Login(); err != nil {
		t.Fatalf("Login returned an error: %v", err)
	}
	if client.Token != "test-token" {
		t.Errorf("Token = %q", client.Token)
	}
}

func TestResolveSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/settings/sources" {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"zeta":{"name":"zeta"},"alpha":{"name":"alpha"}}`))
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL})
	client.Token = "token"
	if err := client.ResolveSource(); err != nil {
		t.Fatalf("ResolveSource returned an error: %v", err)
	}
	if client.Source != "alpha" {
		t.Errorf("Source = %q, want alpha", client.Source)
	}

	invalid := NewClient(Config{URL: server.URL, Source: "missing"})
	invalid.Token = "token"
	if err := invalid.ResolveSource(); err == nil {
		t.Fatal("expected an error for a missing source")
	}

	explicit := NewClient(Config{URL: server.URL, Source: "zeta"})
	explicit.Token = "token"
	if err := explicit.ResolveSource(); err != nil {
		t.Fatalf("ResolveSource(explicit) returned an error: %v", err)
	}
	if explicit.Source != "zeta" {
		t.Errorf("Source = %q, want zeta", explicit.Source)
	}
}

func TestResourceURLAffixesSourceOnlyToResources(t *testing.T) {
	client := NewClient(Config{URL: "http://example.com/files/"})
	client.Source = "media"

	got := client.resourceURL("/api/resources/download", url.Values{"file": {"/a b.txt"}})
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Path != "/files/api/resources/download" {
		t.Errorf("path = %q", parsed.Path)
	}
	if parsed.Query().Get("source") != "media" || parsed.Query().Get("file") != "/a b.txt" {
		t.Errorf("query = %q", parsed.RawQuery)
	}

	got = client.resourceURL("/api/settings/sources", nil)
	if strings.Contains(got, "source=") {
		t.Errorf("settings URL should not include a source: %s", got)
	}
}

func TestMkdirRemoteDoesNotReplaceFiles(t *testing.T) {
	var existingType atomic.Value
	existingType.Store("directory")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// Quantum returns 409 when override is false and the path exists.
			w.WriteHeader(http.StatusConflict)
		case http.MethodGet:
			kind, _ := existingType.Load().(string)
			_, _ = w.Write([]byte(`{"name":"target","type":"` + kind + `"}`))
		default:
			t.Errorf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	client := NewClient(Config{URL: server.URL, Source: "srv"})
	client.Token = "token"

	if err := client.mkdirRemote("/target"); err != nil {
		t.Errorf("existing directory should be accepted: %v", err)
	}

	existingType.Store("text/plain")
	if err := client.mkdirRemote("/target"); err == nil {
		t.Fatal("expected an error when a file exists at the directory path")
	}
}

func TestSplitCommandLine(t *testing.T) {
	args, err := splitCommandLine(`get "a b.txt" '/c d.txt' e\ f ""`)
	if err != nil {
		t.Fatalf("splitCommandLine returned an error: %v", err)
	}
	expected := []string{"get", "a b.txt", "/c d.txt", "e f", ""}
	if len(args) != len(expected) {
		t.Fatalf("args = %#v", args)
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], expected[i])
		}
	}

	if _, err := splitCommandLine(`get "unterminated`); err == nil {
		t.Fatal("expected an error for an unterminated quote")
	}
}

func TestShellPathResolution(t *testing.T) {
	shell := &Shell{
		remoteCWD: "/docs",
		localCWD:  filepath.Join(string(filepath.Separator), "tmp", "work"),
	}
	if got := shell.remotePath("a.txt"); got != "/docs/a.txt" {
		t.Errorf("remotePath(a.txt) = %q", got)
	}
	if got := shell.remotePath("/other/a.txt"); got != "/other/a.txt" {
		t.Errorf("remotePath(/other/a.txt) = %q", got)
	}
	if got := shell.remotePath(".."); got != "/" {
		t.Errorf("remotePath(..) = %q", got)
	}
	if got := shell.localPath("a.txt"); got != filepath.Join(shell.localCWD, "a.txt") {
		t.Errorf("localPath(a.txt) = %q", got)
	}
	if got := shell.localPath(filepath.Join(shell.localCWD, "a.txt")); got != filepath.Join(shell.localCWD, "a.txt") {
		t.Errorf("localPath(absolute) = %q", got)
	}
}

func TestParseShellOptions(t *testing.T) {
	options, rest, err := parseShellOptions([]string{"-l", "-i", `\.log$`, "/docs"}, "li")
	if err != nil {
		t.Fatalf("parseShellOptions returned an error: %v", err)
	}
	if !options.long || options.ignore != `\.log$` {
		t.Errorf("unexpected options: %+v", options)
	}
	if len(rest) != 1 || rest[0] != "/docs" {
		t.Errorf("rest = %#v", rest)
	}

	if _, _, err := parseShellOptions([]string{"-z"}, "i"); err == nil {
		t.Fatal("expected an error for an unsupported flag")
	}
}

func TestShellCompletionHelpers(t *testing.T) {
	if got := shellArgumentIndex([]string{"-r", "-i", "x", "a.txt"}); got != 1 {
		t.Errorf("shellArgumentIndex = %d, want 1", got)
	}
	if got := completionKindFor("get", 0); got != completeRemote {
		t.Errorf("get arg 0 kind = %v", got)
	}
	if got := completionKindFor("get", 1); got != completeLocal {
		t.Errorf("get arg 1 kind = %v", got)
	}
	if got := completionKindFor("put", 0); got != completeLocal {
		t.Errorf("put arg 0 kind = %v", got)
	}
	if got := completionKindFor("put", 1); got != completeRemote {
		t.Errorf("put arg 1 kind = %v", got)
	}
}

func TestShellPrompt(t *testing.T) {
	shell := &Shell{remoteCWD: "/docs", client: &Client{Source: "srv"}}
	if got := shell.prompt(); got != "fbcli[srv]:/docs> " {
		t.Errorf("prompt = %q", got)
	}
}
