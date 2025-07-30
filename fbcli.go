package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/term"
)

const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36"

var version = "dev" // this will be set by the build process

// exitWithError prints an error message to stderr and exits with code 1
func exitWithError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(1)
}

type Config struct {
	URL      string
	Username string
	Password string
}

type Client struct {
	Config Config
	Token  string
}

func main() {
	progName := filepath.Base(os.Args[0])
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> [arguments...]\n", progName)
		usage(progName)
	}

	cfg := Config{
		URL:      os.Getenv("FILEBROWSER_URL"),
		Username: os.Getenv("FILEBROWSER_USERNAME"),
		Password: os.Getenv("FILEBROWSER_PASSWORD"),
	}
	client := &Client{Config: cfg}

	cmd := os.Args[1]
	args := os.Args[2:]

	if err := client.getCredentials(); err != nil {
		os.Exit(1)
	}

	if cmd == "show" {
		fmt.Printf("Version: %s\nURL: %s\nUsername: %s\n", version, client.Config.URL, client.Config.Username)
		os.Exit(0)
	}

	if err := client.Login(); err != nil {
		os.Exit(1)
	}

	// Commands that take two path arguments
	twoPathCommands := map[string]func(string, string){
		"rename": client.Rename,
		"mv":     client.Rename,
	}

	ignoreName := ""
	zipFlag := false
	scriptFlag := false
	listFlag := false
	newArgs := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "-i" && i+1 < len(args) {
			ignoreName = args[i+1]
			i++
		} else if args[i] == "-z" {
			zipFlag = true
		} else if args[i] == "-s" || args[i] == "--script" {
			scriptFlag = true
		} else if args[i] == "-l" {
			listFlag = true
		} else {
			newArgs = append(newArgs, args[i])
		}
	}

	if cmd == "ls" || cmd == "list" || cmd == "dir" {
		remotePath := "/"
		if len(newArgs) > 0 {
			remotePath = strings.Join(newArgs, " ")
		}
		if listFlag || cmd == "list" || cmd == "dir" {
			// Detailed list view (like ls -l)
			client.ListIgnore(remotePath, ignoreName)
		} else {
			// Regular ls view (multi-column or script mode)
			client.LsIgnoreScript(remotePath, ignoreName, scriptFlag)
		}
	} else if cmd == "rm" || cmd == "delete" {
		if len(newArgs) < 1 {
			usage(progName)
		}
		for _, path := range newArgs {
			if ignoreName != "" {
				client.DeleteIgnore(path, ignoreName)
			} else {
				client.Delete(path)
			}
		}
	} else if cmd == "mkdir" || cmd == "md" {
		if len(newArgs) < 1 {
			usage(progName)
		}
		for _, path := range newArgs {
			client.Mkdir(path)
		}
	} else if cmd == "upload" || cmd == "up" {
		if len(newArgs) < 1 || len(newArgs) > 2 {
			usage(progName)
		}
		remotePath := "/"
		if len(newArgs) == 2 {
			remotePath = newArgs[1]
		}
		client.UploadIgnore(newArgs[0], remotePath, ignoreName)
	} else if cmd == "download" || cmd == "down" || cmd == "dl" { // Special handling for download to allow optional localPath
		if zipFlag && ignoreName != "" {
			fmt.Fprintln(os.Stderr, "-z (zip) and -i (ignore) cannot be used together.")
			usage(progName)
		}
		if len(newArgs) < 1 || len(newArgs) > 2 {
			usage(progName)
		}
		remotePath := newArgs[0]
		localPath := ""
		if len(newArgs) == 2 {
			localPath = newArgs[1]
		} else {
			// If localPath is not provided...
			if zipFlag {
				// for zip downloads, default to basename.zip
				localPath = filepath.Base(remotePath) + ".zip"
			} else {
				// for regular downloads, use the base name of the remotePath
				localPath = filepath.Base(remotePath)
			}
		}
		if zipFlag {
			// Determine zip file path logic
			zipPath := localPath
			remoteBase := filepath.Base(remotePath)
			// Helper to check for zip/dir conflict and add suffix if needed
			nextAvailableZip := func(base string, dir string) string {
				name := base + ".zip"
				candidate := filepath.Join(dir, name)
				i := 1
				for {
					if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
						// Conflict: a directory exists with this name, try next
						name = base + fmt.Sprintf("-%d.zip", i)
						candidate = filepath.Join(dir, name)
						i++
					} else {
						break
					}
				}
				return candidate
			}
			if zipPath == "" {
				zipPath = remoteBase + ".zip"
				if fi, err := os.Stat(zipPath); err == nil && fi.IsDir() {
					// Conflict: zipPath is a directory, add suffix
					zipPath = nextAvailableZip(remoteBase, ".")
				}
			} else if strings.HasSuffix(zipPath, string(os.PathSeparator)) || (func() bool { info, err := os.Stat(zipPath); return err == nil && info.IsDir() })() {
				zipPath = nextAvailableZip(remoteBase, zipPath)
			} else if !strings.HasSuffix(zipPath, ".zip") {
				// If not ending with .zip and not a dir, treat as file name
				zipPath = zipPath + ".zip"
				if fi, err := os.Stat(zipPath); err == nil && fi.IsDir() {
					// Conflict: zipPath is a directory, add suffix
					dir := filepath.Dir(zipPath)
					base := strings.TrimSuffix(filepath.Base(zipPath), ".zip")
					zipPath = nextAvailableZip(base, dir)
				}
			}
			client.Download(remotePath, zipPath)
		} else {
			client.DownloadIgnore(remotePath, localPath, ignoreName)
		}
	} else if fn, ok := twoPathCommands[cmd]; ok {
		if len(args) != 2 {
			usage(progName)
		}
		fn(args[0], args[1])
	} else if cmd == "syncto" || cmd == "to" {
		if len(newArgs) != 2 {
			usage(progName)
		}
		client.SyncToIgnore(newArgs[0], newArgs[1], ignoreName)
	} else if cmd == "syncfrom" || cmd == "from" {
		if len(newArgs) != 2 {
			usage(progName)
		}
		client.SyncFromIgnore(newArgs[0], newArgs[1], ignoreName)
	} else {
		usage(progName)
	}

}

// SyncToIgnore is like SyncTo but ignores files/dirs matching ignoreName
func (c *Client) SyncToIgnore(localPath, remotePath, ignoreName string) {
	fmt.Printf("Syncing from local '%s' to remote '%s' (ignoring '%s')\n", localPath, remotePath, ignoreName)
	info, err := os.Stat(localPath)
	if err != nil {
		exitWithError("Error accessing local path: %v", err)
	}
	if !info.IsDir() {
		var ignoreRegex *regexp.Regexp
		if ignoreName != "" {
			var err error
			ignoreRegex, err = regexp.Compile(ignoreName)
			if err != nil {
				exitWithError("Invalid ignore regex: %v", err)
			}
		}
		if ignoreRegex != nil && shouldIgnoreRegex(info.Name(), ignoreRegex) {
			fmt.Printf("Ignoring file: %s\n", info.Name())
			return
		}
		remoteFilePath := path.Join(remotePath, info.Name())
		c.syncFileToRemote(localPath, remoteFilePath, info)
		return
	}
	localPaths := make(map[string]os.FileInfo)
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		var err error
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {

			localPaths[relPath] = info
			return nil
		}
		if ignoreRegex != nil {
			pathParts := strings.Split(relPath, "/")
			for _, part := range pathParts {
				if shouldIgnoreRegex(part, ignoreRegex) {
					// If this is a directory, skip the whole subtree
					if info.IsDir() {

						return filepath.SkipDir
					}
					// If this is a file, skip the file

					return nil
				}
			}
		}

		localPaths[relPath] = info
		return nil
	})
	if walkErr != nil {
		exitWithError("Error walking local path: %v", walkErr)
	}
	for relPath, info := range localPaths {
		if relPath == "." {
			continue
		}
		if ignoreRegex != nil {
			pathParts := strings.Split(relPath, "/")
			skip := false
			for _, part := range pathParts {
				if shouldIgnoreRegex(part, ignoreRegex) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		remoteItemPath := path.Join(remotePath, relPath)
		if info.IsDir() {
			c.Mkdir(remoteItemPath)
		} else {
			c.syncFileToRemote(filepath.Join(localPath, relPath), remoteItemPath, info)
		}
	}
	var walkRemote func(string, string)
	walkRemote = func(remoteDir, localDir string) {
		remoteItems, err := c.listRemote(remoteDir)
		if err != nil {
			return
		}
		for _, item := range remoteItems {
			if ignoreRegex != nil && shouldIgnoreRegex(item.Name, ignoreRegex) {
				continue
			}
			rel := path.Join(strings.TrimPrefix(remoteDir, remotePath), item.Name)
			rel = strings.TrimPrefix(rel, "/")
			localItem, exists := localPaths[rel]
			remoteItemPath := path.Join(remoteDir, item.Name)
			if !exists {
				if item.IsDir {
					fmt.Printf("Deleting remote directory not in source: %s\n", remoteItemPath)
					c.Delete(remoteItemPath)
				} else {
					fmt.Printf("Deleting remote file not in source: %s\n", remoteItemPath)
					c.Delete(remoteItemPath)
				}
			} else if item.IsDir && localItem.IsDir() {
				walkRemote(remoteItemPath, filepath.Join(localDir, item.Name))
			}
		}
	}
	walkRemote(remotePath, localPath)
}

// SyncFromIgnore is like SyncFrom but ignores files/dirs matching ignoreName
func (c *Client) SyncFromIgnore(remotePath, localPath, ignoreName string) {
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		var err error
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		exitWithError("Error checking remote path type: %v", err)
	}
	if !isDir {
		if ignoreRegex != nil && shouldIgnoreRegex(path.Base(remotePath), ignoreRegex) {
			fmt.Printf("Ignoring file: %s\n", path.Base(remotePath))
			return
		}
		c.syncFileFromRemote(remotePath, localPath)
		return
	}
	fmt.Printf("Syncing remote directory '%s' to local '%s' (ignoring '%s')\n", remotePath, localPath, ignoreName)
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		exitWithError("Failed to create directory %s: %v", localPath, err)
	}
	remoteItems := make(map[string]RemoteItem)
	var collectRemote func(string, string, bool)
	collectRemote = func(rPath, relBase string, isTopLevel bool) {
		items, err := c.listRemote(rPath)
		if err != nil {
			return
		}
		for _, item := range items {
			if isTopLevel && ignoreRegex != nil && shouldIgnoreRegex(item.Name, ignoreRegex) {
				continue
			}
			rel := path.Join(relBase, item.Name)
			remoteItems[rel] = item
			if item.IsDir {
				collectRemote(path.Join(rPath, item.Name), rel, false)
			}
		}
	}
	collectRemote(remotePath, "", true)
	localItems := make(map[string]os.FileInfo)
	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		// Only ignore at the top level
		if relPath != "." && strings.Count(relPath, "/") == 0 && ignoreRegex != nil && shouldIgnoreRegex(info.Name(), ignoreRegex) {
			return nil // skip
		}
		localItems[relPath] = info
		return nil
	})
	if walkErr != nil {
		exitWithError("Error walking local path: %v", walkErr)
	}
	for rel, item := range remoteItems {
		exists := false
		if _, ok := localItems[rel]; ok {
			exists = true
		}
		newRemotePath := path.Join(remotePath, rel)
		newLocalPath := filepath.Join(localPath, rel)
		if item.IsDir {
			if !exists {
				if err := os.MkdirAll(newLocalPath, os.ModePerm); err == nil {
					fmt.Printf("Directory created: %s\n", newLocalPath)
				}
			}
			c.SyncFromIgnore(newRemotePath, newLocalPath, ignoreName)
		} else {
			c.syncFileFromRemote(newRemotePath, newLocalPath)
		}
	}
	for rel, info := range localItems {
		if rel == "." {
			continue
		}
		if _, exists := remoteItems[rel]; !exists {
			// Only ignore at the top level
			if strings.Count(rel, "/") == 0 && ignoreRegex != nil && shouldIgnoreRegex(info.Name(), ignoreRegex) {
				continue
			}
			localPathToDelete := filepath.Join(localPath, rel)
			if info.IsDir() {
				fmt.Printf("Deleting local directory not in remote: %s\n", localPathToDelete)
				if err := os.RemoveAll(localPathToDelete); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting directory: %v\n", err)
				}
			} else {
				fmt.Printf("Deleting local file not in remote: %s\n", localPathToDelete)
				if err := os.Remove(localPathToDelete); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting file: %v\n", err)
				}
			}
		}
	}
}

func (c *Client) getCredentials() error {
	if c.Config.URL == "" {
		fmt.Print("Enter File Browser URL: ")
		if _, err := fmt.Scanln(&c.Config.URL); err != nil {
			return err
		}
	}

	if c.Config.Username == "" {
		fmt.Print("Enter Username: ")
		if _, err := fmt.Scanln(&c.Config.Username); err != nil {
			return err
		}
	}

	if c.Config.Password == "" {
		fmt.Print("Enter Password: ")
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			reader := bufio.NewReader(os.Stdin)
			password, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			c.Config.Password = strings.TrimSpace(password)
			return nil
		}

		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer func() {
			if err := term.Restore(fd, oldState); err != nil {
				fmt.Fprintf(os.Stderr, "Error restoring terminal state: %v\n", err)
			}
		}()

		var password []byte
		var backspace = []byte{' ', ' ', ' '}
		for {
			var buf [1]byte
			n, err := os.Stdin.Read(buf[:])
			if err != nil || n == 0 {
				return err
			}
			char := buf[0]
			if char == '\r' || char == '\n' {
				fmt.Print("\r\n")
				break
			}
			switch char {
			case 8, 127:
				if len(password) > 0 {
					password = password[:len(password)-1]
					if _, err := os.Stdout.Write(backspace); err != nil {
						fmt.Fprintf(os.Stderr, "Error writing backspace: %v\n", err)
					}
				}
			case 3:
				return fmt.Errorf("interrupted")
			default:
				password = append(password, char)
				fmt.Print("*")
			}
		}
		c.Config.Password = string(password)
	}

	return nil
}

func usage(progName string) {
	fmt.Printf("%s version %s\n", progName, version)
	fmt.Printf("Usage: %s <command> [arguments...]\n", progName)
	fmt.Print(`
Commands:
  ls [-i ignore] [-l] [-s] [remote_path]       List files/directories (optional remote_path)
                                               -l: detailed view with sizes and dates
                                               -s: script-friendly output (one per line, no colors)
  list, dir [-i ignore] [remote_path]          List detailed info (like ls -l) (optional remote_path)
  upload, up [-i ignore] <local_path> [remote_dir] Upload a file or directory (optional remote_dir)
  download, down, dl [-i ignore] [-z] <remote_path> [local_path] Download a file or directory (optional local_path)
  mkdir, md <remote_path>...               Create one or more directories
  rm, delete [-i ignore] <remote_path>...  Delete one or more files or directories
  rename, mv <old_path> <new_path>         Rename a file or directory
  show                                   Show the current configuration
  syncto, to [-i ignore] <local_path> <remote_path>   Sync files from a local path to a remote path
  syncfrom, from [-i ignore] <remote_path> <local_path> Sync files from a remote path to a local path
`)
	os.Exit(1)
}

func (c *Client) ShowConfig() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("URL: %s\n", c.Config.URL)
	fmt.Printf("Username: %s\n", c.Config.Username)
	fmt.Printf("Password: %s\n", redactPassword(c.Config.Password))
}

func redactPassword(s string) string {
	length := len(s)
	if length == 0 {
		return ""
	}
	if length == 1 {
		return "*"
	}
	if length == 2 {
		return "**"
	}
	return string(s[0]) + strings.Repeat("*", length-2) + string(s[length-1])
}

func (c *Client) Login() error {
	loginURL := c.Config.URL + "/api/login"
	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`,
		c.Config.Username, c.Config.Password)
	client := &http.Client{}
	req, err := http.NewRequest("POST", loginURL, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	// The response body is the JWT token (as in filebrowser_client.sh)
	b, _ := io.ReadAll(resp.Body)
	c.Token = strings.TrimSpace(string(b))
	return nil
}

func (c *Client) apiRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	u, _ := url.Parse(c.Config.URL + path)
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	if c.Token != "" {
		req.AddCookie(&http.Cookie{Name: "auth", Value: c.Token})
		req.Header.Set("X-Auth", c.Token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{}
	return client.Do(req)
}

func (c *Client) Ls(remotePath string) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("API error %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []struct {
			Name     string
			IsDir    bool
			Modified string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		exitWithError("Failed to decode response: %v", err)
	}
	// Deduplicate by normalized name, keep most recent Modified
	type entry struct {
		Name     string
		IsDir    bool
		Modified string
	}
	dedup := make(map[string]entry)
	for _, item := range data.Items {
		norm := strings.TrimRight(item.Name, "\r\n")
		if strings.TrimSpace(norm) == "" {
			continue // skip blank/ghost entries after normalization
		}
		if e, ok := dedup[norm]; !ok || item.Modified > e.Modified {
			dedup[norm] = entry{item.Name, item.IsDir, item.Modified}
		}
	}
	// Sort by Modified descending, then by Name
	sorted := make([]entry, 0, len(dedup))
	for _, v := range dedup {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Modified == sorted[j].Modified {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Modified > sorted[j].Modified
	})
	// Print like bash: tab-separated, trailing slash for dirs
	const (
		colorBlue  = "\033[1;34m"
		colorReset = "\033[0m"
	)
	// Prepare names and calculate max width
	names := make([]string, len(sorted))
	maxLen := 0
	for i, e := range sorted {
		name := e.Name
		name = strings.ReplaceAll(name, "\n", "")
		name = strings.ReplaceAll(name, "\r", "")
		if e.IsDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		if e.IsDir {
			name = colorBlue + name + colorReset
		}
		names[i] = name
		// Visible length (strip color codes for width)
		visible := len([]rune(stripANSICodes(name)))
		if visible > maxLen {
			maxLen = visible
		}
	}
	if maxLen < 16 {
		maxLen = 16
	}
	// Get terminal width (default 80)
	width := 80
	if w, ok := getTerminalWidth(); ok {
		width = w
	}
	colWidth := maxLen + 2
	cols := width / colWidth
	if cols < 1 {
		cols = 1
	}
	rows := (len(names) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			i := c*rows + r
			if i >= len(names) {
				continue
			}
			name := names[i]
			fmt.Printf("%-*s", colWidth+len(name)-len(stripANSICodes(name)), name)
		}
		fmt.Println()
	}

	// Helper to strip ANSI color codes
}

// stripANSICodes removes ANSI escape sequences from a string
func stripANSICodes(s string) string {
	res := make([]rune, 0, len(s))
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		res = append(res, r)
	}
	return string(res)
}

// getTerminalWidth tries to get the terminal width, returns (width, ok)
func getTerminalWidth() (int, bool) {
	// Try golang.org/x/term for portable terminal width
	fd := int(os.Stdout.Fd())
	width, _, err := term.GetSize(fd)
	if err == nil && width > 0 {
		return width, true
	}
	// Fallback to COLUMNS env
	if colStr := os.Getenv("COLUMNS"); colStr != "" {
		var cols int
		_, err := fmt.Sscanf(colStr, "%d", &cols)
		if err == nil && cols > 0 {
			return cols, true
		}
	}
	return 80, false
}

func (c *Client) List(remotePath string) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("API error %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []struct {
			Name     string
			IsDir    bool
			Size     int64
			Modified string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		exitWithError("Failed to decode response: %v", err)
	}
	// Deduplicate by normalized name, keep most recent Modified
	type entry struct {
		Name     string
		IsDir    bool
		Modified string
		Size     int64
	}
	dedup := make(map[string]entry)
	maxName := 4 // min width for 'Name'
	for _, item := range data.Items {
		norm := strings.TrimRight(item.Name, "\r\n")
		if strings.TrimSpace(norm) == "" {
			continue // skip blank/ghost entries after normalization
		}
		if e, ok := dedup[norm]; !ok || item.Modified > e.Modified {
			dedup[norm] = entry{item.Name, item.IsDir, item.Modified, item.Size}
		}
		if l := len(item.Name); l > maxName {
			maxName = l
		}
	}
	if maxName > 60 {
		maxName = 60
	} else if maxName < 30 {
		maxName = 30
	}
	// Sort by Modified descending, then by Name
	sorted := make([]entry, 0, len(dedup))
	for _, v := range dedup {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Modified == sorted[j].Modified {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Modified > sorted[j].Modified
	})
	// Print header
	pad := maxName
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%-*s %-19s %-8s\n", pad, "Name", "Modified", "Size")
	fmt.Printf("%-*s %-19s %-8s\n", pad, strings.Repeat("-", pad), strings.Repeat("-", 19), strings.Repeat("-", 8))
	const (
		colorBlue  = "\033[1;34m"
		colorReset = "\033[0m"
	)
	for _, e := range sorted {
		name := e.Name
		// Remove newlines and carriage returns from name
		name = strings.ReplaceAll(name, "\n", "")
		name = strings.ReplaceAll(name, "\r", "")
		if e.IsDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		// Format date like bash: YYYY-MM-DD HH:MM:SS
		date := e.Modified
		if len(date) > 19 && strings.Contains(date, "T") {
			date = strings.Replace(date[:19], "T", " ", 1)
		} else if len(date) > 19 {
			date = date[:19]
		}
		visibleLen := len([]rune(name))
		if e.IsDir {
			colorName := colorBlue + name + colorReset
			pad := maxName - visibleLen
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("%s%s %-19s %-8d\n", colorName, strings.Repeat(" ", pad), date, e.Size)
		} else {
			pad := maxName
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("%-*s %-19s %-8d\n", pad, name, date, e.Size)
		}
	}
}

func (c *Client) Mkdir(remotePath string) {
	cleanPath := strings.TrimSpace(remotePath)
	if cleanPath == "/" || cleanPath == "" {
		exitWithError("Cannot create root directory.")
	}
	trimmed := strings.Trim(cleanPath, "/")
	if trimmed == "" {
		exitWithError("Invalid directory name.")
	}
	// Use POST, no trailing slash, set browser-like headers
	encoded := encodeSegments(remotePath)
	url := "/api/resources" + encoded + "/?override=false"
	headers := map[string]string{
		"Content-Type":    "text/plain; charset=UTF-8",
		"Content-Length":  "0",
		"Accept-Language": "en-US,en;q=0.9",
		"Origin":          c.Config.URL,
		"Referer":         c.Config.URL + "/files/",
	}
	resp, err := c.apiRequest("POST", url, nil, headers)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("Directory creation failed for '%s'. Server responded with HTTP %d.\n%s", remotePath, resp.StatusCode, string(b))
	}
	fmt.Printf("Directory created: %s\n", remotePath)
}

func (c *Client) Delete(remotePath string) {
	encoded := encodePathPreserveSlash(remotePath)
	if encoded == "" {
		exitWithError("Invalid path.")
	}

	resp, err := c.apiRequest("DELETE", "/api/resources"+encoded, nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("Delete failed: %s", string(b))
	}
	fmt.Println("Deletion complete.")
}

// DeleteIgnore deletes files and directories, ignoring entries matching ignoreName (regex)
func (c *Client) DeleteIgnore(remotePath, ignoreName string) {
	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		exitWithError("Error checking remote path: %v", err)
	}

	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		var err error
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}

	if !isDir {
		// It's a file
		if ignoreRegex != nil && shouldIgnoreRegex(path.Base(remotePath), ignoreRegex) {
			fmt.Printf("Ignoring file: %s\n", remotePath)
		} else {
			c.Delete(remotePath)
		}
		return
	}

	// It's a directory, delete its contents recursively, honoring ignore
	fmt.Printf("Deleting contents of '%s' (ignoring '%s')\n", remotePath, ignoreName)
	c.deleteRecursive(remotePath, ignoreRegex)
}

// deleteRecursive is a helper to delete directory contents, honoring an ignore regex
func (c *Client) deleteRecursive(remoteDirPath string, ignoreRegex *regexp.Regexp) {
	items, err := c.listRemote(remoteDirPath)
	if err != nil {
		exitWithError("Failed to list remote directory %s: %v", remoteDirPath, err)
	}

	for _, item := range items {
		itemPath := path.Join(remoteDirPath, item.Name)
		if ignoreRegex != nil && shouldIgnoreRegex(item.Name, ignoreRegex) {
			fmt.Printf("Ignoring: %s\n", itemPath)
			continue
		}

		if item.IsDir {
			// Recursively delete contents of subdirectory first
			c.deleteRecursive(itemPath, ignoreRegex)
		}

		// Delete the file or the now-empty directory
		c.Delete(itemPath)
	}
}

func (c *Client) Rename(oldPath, newPath string) {
	oldP := encodePathPreserveSlash(oldPath)
	newP := encodePathPreserveSlash(newPath)
	url := fmt.Sprintf("/api/resources%s?action=rename&destination=%s&override=false&rename=false", oldP, newP)
	resp, err := c.apiRequest("PATCH", url, nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("Rename failed: %s", string(b))
	}
	fmt.Println("Rename complete.")
}

func (c *Client) Upload(localPath, remoteDir string) {
	info, err := os.Stat(localPath)
	if err != nil {
		exitWithError("Error accessing local path: %v", err)
	}

	if !info.IsDir() {
		err := c.uploadFile(localPath, remoteDir)
		if err != nil {
			exitWithError("Upload failed: %v", err)
		} else {
			fmt.Println("Upload complete.")
		}
		return
	}

	fmt.Printf("Uploading directory '%s' to '%s'\n", localPath, remoteDir)

	localDirName := filepath.Base(localPath)
	fullRemoteDir := path.Join(remoteDir, localDirName)

	c.Mkdir(fullRemoteDir)

	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		remoteItemPath := path.Join(fullRemoteDir, relPath)

		if info.IsDir() {
			if currentLocalPath != localPath {
				fmt.Printf("Creating remote directory: %s\n", remoteItemPath)
				c.Mkdir(remoteItemPath)
			}
		} else {
			remoteParentDir := path.Dir(remoteItemPath)
			fmt.Printf("Uploading file %s to %s\n", currentLocalPath, remoteParentDir)
			err := c.uploadFile(currentLocalPath, remoteParentDir)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", currentLocalPath, err)
			}
		}
		return nil
	})
	if walkErr != nil {
		exitWithError("Error during directory upload: %v", walkErr)
	}

	fmt.Println("Directory upload complete.")
}

func closeFileWithDebug(file *os.File, location string) {
	// fmt.Fprintf(os.Stderr, "Closing file at %s\n", location)
	if err := file.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing file at %s: %v\n", location, err)
	}
}

func (c *Client) uploadFile(localPath, remoteDir string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	filename := filepath.Base(localPath)
	remoteDirEnc := encodePathPreserveSlash(remoteDir)
	filenameEnc := encodePathPreserveSlash(filename)
	url := fmt.Sprintf("/api/resources%s/%s?override=true", remoteDirEnc, filenameEnc)
	headers := map[string]string{"Content-Type": "application/octet-stream"}

	// First attempt
	resp, err := c.apiRequest("POST", url, file, headers)
	if err != nil {
		return err
	}

	// If 404, directory may not exist. Create it and retry.
	if resp.StatusCode == 404 {
		_ = resp.Body.Close()
		c.Mkdir(remoteDir)
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek file: %w", err)
		}
		resp, err = c.apiRequest("POST", url, file, headers)
		if err != nil {
			return err
		}
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		return fmt.Errorf("server response %d", resp.StatusCode)
	}

	// Read and discard the response body to ensure the upload is complete
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) SyncTo(localPath, remotePath string) {
	fmt.Printf("Syncing from local '%s' to remote '%s'\n", localPath, remotePath)
	info, err := os.Stat(localPath)
	if err != nil {
		exitWithError("Error accessing local path: %v", err)
	}

	if !info.IsDir() {
		// It's a file
		remoteFilePath := path.Join(remotePath, info.Name())
		c.syncFileToRemote(localPath, remoteFilePath, info)
		return
	}

	// It's a directory
	// Collect all local relative paths
	localPaths := make(map[string]os.FileInfo)
	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		localPaths[relPath] = info
		return nil
	})
	if walkErr != nil {
		exitWithError("Error walking local path: %v", walkErr)
	}

	// Sync local to remote (create/update)
	for relPath, info := range localPaths {
		remoteItemPath := path.Join(remotePath, relPath)
		if info.IsDir() {
			c.Mkdir(remoteItemPath)
		} else {
			c.syncFileToRemote(filepath.Join(localPath, relPath), remoteItemPath, info)
		}
	}

	// Delete remote files/dirs not in local
	var walkRemote func(string, string)
	walkRemote = func(remoteDir, localDir string) {
		remoteItems, err := c.listRemote(remoteDir)
		if err != nil {
			return
		}
		for _, item := range remoteItems {
			rel := path.Join(strings.TrimPrefix(remoteDir, remotePath), item.Name)
			rel = strings.TrimPrefix(rel, "/")
			localItem, exists := localPaths[rel]
			remoteItemPath := path.Join(remoteDir, item.Name)
			if !exists {
				// Not in local, delete from remote
				if item.IsDir {
					fmt.Printf("Deleting remote directory not in source: %s\n", remoteItemPath)
					c.Delete(remoteItemPath)
				} else {
					fmt.Printf("Deleting remote file not in source: %s\n", remoteItemPath)
					c.Delete(remoteItemPath)
				}
			} else if item.IsDir && localItem.IsDir() {
				walkRemote(remoteItemPath, filepath.Join(localDir, item.Name))
			}
		}
	}
	walkRemote(remotePath, localPath)
}

func (c *Client) syncFileToRemote(localPath, remotePath string, localFileInfo os.FileInfo) {
	remoteItems, err := c.listRemote(path.Dir(remotePath))
	if err != nil {
		// Assume directory doesn't exist, so upload
		if err := c.uploadFile(localPath, path.Dir(remotePath)); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to upload file: %v\n", err)
		}
		return
	}

	var remoteItem *RemoteItem
	for i, item := range remoteItems {
		if item.Name == path.Base(remotePath) {
			remoteItem = &remoteItems[i]
			break
		}
	}

	if remoteItem != nil {
		// File exists on remote, compare
		if localFileInfo.Size() != remoteItem.Size {
			fmt.Printf("File size mismatch for %s. Uploading.\n", localPath)
			if err := c.uploadFile(localPath, path.Dir(remotePath)); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to upload file: %v\n", err)
			}
			return
		}

		if localFileInfo.Size() < 1024*1024 { // 1MB
			localHash, err := getLocalFileHash(localPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error hashing local file %s: %v\n", localPath, err)
				return
			}
			localHashNorm := strings.ToLower(strings.TrimSpace(localHash))
			remoteHash, err := c.getRemoteFileHash(remotePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching remote hash for %s: %v\n", remotePath, err)
				// fallback: assume not in sync
				fmt.Printf("File hash mismatch for %s. Uploading.\n", localPath)
				if err := c.uploadFile(localPath, path.Dir(remotePath)); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to upload file: %v\n", err)
				}
				return
			}
			remoteHashNorm := strings.ToLower(strings.TrimSpace(remoteHash))
			if localHashNorm != remoteHashNorm {
				fmt.Printf("File hash mismatch for %s. Uploading.\n", localPath)
				if err := c.uploadFile(localPath, path.Dir(remotePath)); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to upload file: %v\n", err)
				}
			} else {
				fmt.Printf("File %s is already in sync.\n", localPath)
			}
		} else {
			fmt.Printf("File %s is already in sync (size match, not hashing >1MB).\n", localPath)
		}
	} else {
		// File does not exist on remote, upload
		if err := c.uploadFile(localPath, path.Dir(remotePath)); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to upload file: %v\n", err)
		}
	}
}

func (c *Client) SyncFrom(remotePath, localPath string) {
	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		exitWithError("Error checking remote path type: %v", err)
	}

	if !isDir {
		// It's a file
		c.syncFileFromRemote(remotePath, localPath)
		return
	}

	// It's a directory
	fmt.Printf("Syncing remote directory '%s' to local '%s'\n", remotePath, localPath)
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		exitWithError("Failed to create directory %s: %v", localPath, err)
	}

	// Collect all remote items
	remoteItems := make(map[string]RemoteItem)
	var collectRemote func(string, string)
	collectRemote = func(rPath, relBase string) {
		items, err := c.listRemote(rPath)
		if err != nil {
			return
		}
		for _, item := range items {
			rel := path.Join(relBase, item.Name)
			remoteItems[rel] = item
			if item.IsDir {
				collectRemote(path.Join(rPath, item.Name), rel)
			}
		}
	}
	collectRemote(remotePath, "")

	// Collect all local items
	localItems := make(map[string]os.FileInfo)
	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		localItems[relPath] = info
		return nil
	})
	if walkErr != nil {
		exitWithError("Error walking local path: %v", walkErr)
	}

	// Download or update files/dirs from remote
	for rel, item := range remoteItems {
		exists := false
		if _, ok := localItems[rel]; ok {
			exists = true
		}
		newRemotePath := path.Join(remotePath, rel)
		newLocalPath := filepath.Join(localPath, rel)
		if item.IsDir {
			if !exists {
				if err := os.MkdirAll(newLocalPath, os.ModePerm); err == nil {
					fmt.Printf("Directory created: %s\n", newLocalPath)
				}
			}
			c.SyncFrom(newRemotePath, newLocalPath)
		} else {
			c.syncFileFromRemote(newRemotePath, newLocalPath)
		}
	}

	// Delete local files/dirs not in remote
	for rel, info := range localItems {
		if rel == "." {
			continue
		}
		if _, exists := remoteItems[rel]; !exists {
			localPathToDelete := filepath.Join(localPath, rel)
			if info.IsDir() {
				fmt.Printf("Deleting local directory not in remote: %s\n", localPathToDelete)
				if err := os.RemoveAll(localPathToDelete); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting directory: %v\n", err)
				}
			} else {
				fmt.Printf("Deleting local file not in remote: %s\n", localPathToDelete)
				if err := os.Remove(localPathToDelete); err != nil {
					fmt.Fprintf(os.Stderr, "Error deleting file: %v\n", err)
				}
			}
		}
	}
}

func (c *Client) syncFileFromRemote(remotePath, localPath string) {
	remoteItems, err := c.listRemote(path.Dir(remotePath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list remote directory %s: %v\n", path.Dir(remotePath), err)
		return
	}

	var remoteItem *RemoteItem
	for i, item := range remoteItems {
		if item.Name == path.Base(remotePath) {
			remoteItem = &remoteItems[i]
			break
		}
	}

	if remoteItem == nil {
		fmt.Fprintf(os.Stderr, "Remote file %s not found.\n", remotePath)
		return
	}

	localFileInfo, err := os.Stat(localPath)
	if err == nil {
		// File exists locally, compare
		if localFileInfo.Size() != remoteItem.Size {
			fmt.Printf("File size mismatch for %s. Downloading.\n", remotePath)
			if err := c.downloadFile(remotePath, localPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
			}
			return
		}

		if localFileInfo.Size() < 1024*1024 { // 1MB
			localHash, err := getLocalFileHash(localPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error hashing local file %s: %v\n", localPath, err)
				return
			}
			localHashNorm := strings.ToLower(strings.TrimSpace(localHash))
			remoteHash, err := c.getRemoteFileHash(remotePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error fetching remote hash for %s: %v\n", remotePath, err)
				// fallback: assume not in sync
				fmt.Printf("File hash mismatch for %s. Downloading.\n", remotePath)
				if err := c.downloadFile(remotePath, localPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
				}
				return
			}
			remoteHashNorm := strings.ToLower(strings.TrimSpace(remoteHash))
			if localHashNorm != remoteHashNorm {
				fmt.Printf("File hash mismatch for %s. Downloading.\n", remotePath)
				if err := c.downloadFile(remotePath, localPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
				}
			} else {
				fmt.Printf("File %s is already in sync.\n", localPath)
			}
		} else {
			fmt.Printf("File %s is already in sync (size match, not hashing >1MB).\n", localPath)
		}
	} else {
		// File does not exist locally, download
		if err := c.downloadFile(remotePath, localPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
		}
	}
}

// getRemoteFileHash fetches the SHA256 hash of a remote file using the File Browser API
func (c *Client) getRemoteFileHash(remotePath string) (string, error) {
	apiURL := "/api/resources" + encodePathPreserveSlash(remotePath) + "?checksum=sha256"
	headers := map[string]string{
		"Accept":          "*/*",
		"Accept-Language": "en-US,en;q=0.9",
	}
	resp, err := c.apiRequest("GET", apiURL, nil, headers)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	rawBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(rawBody))
	}
	var data struct {
		Checksums map[string]string `json:"checksums"`
	}
	err = json.Unmarshal(rawBody, &data)
	if err != nil {
		return "", fmt.Errorf("failed to decode checksum response: %w", err)
	}
	hash := ""
	if data.Checksums != nil {
		hash = data.Checksums["sha256"]
	}
	return hash, nil
}

func getLocalFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer closeFileWithDebug(file, "getLocalFileHash")

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

type RemoteItem struct {
	Name     string `json:"name"`
	IsDir    bool   `json:"isDir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

func (c *Client) listRemote(remotePath string) ([]RemoteItem, error) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []RemoteItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		// It might be a single file response, which is not a list.
		// The caller should have checked with isRemotePathDir.
		// This indicates an issue if called on a file.
		return nil, fmt.Errorf("failed to decode directory listing for '%s': %w", remotePath, err)
	}
	return data.Items, nil
}

func (c *Client) downloadFile(remotePath, localPath string) error {
	fmt.Printf("Downloading file '%s' to '%s'\n", remotePath, localPath)
	downloadURL := "/api/raw" + encodePathPreserveSlash(remotePath)

	resp, err := c.apiRequest("GET", downloadURL, nil, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download failed: %s", string(b))
	}

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create parent directory %s: %w", localDir, err)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer closeFileWithDebug(out, "downloadFile")

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("error saving downloaded file: %w", err)
	}
	fmt.Println("Download complete.")
	return nil
}

func (c *Client) isRemotePathDir(remotePath string) (bool, error) {
	path := encodePathPreserveSlash(remotePath)
	resp, err := c.apiRequest("GET", "/api/resources"+path, nil, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != 200 {
		// The filebrowser API returns 404 for not found
		if resp.StatusCode == http.StatusNotFound {
			return false, fmt.Errorf("remote path '%s' not found (404)", remotePath)
		}
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}

	// Read body into a buffer so we can try decoding it multiple times
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	// First, try to decode as a single resource object
	var singleResource struct {
		IsDir bool `json:"isDir"`
	}
	if err := json.Unmarshal(bodyBytes, &singleResource); err == nil {
		return singleResource.IsDir, nil
	}

	// If that fails, try to decode as a directory listing (an object with an "items" array)
	var dirListing struct {
		Items []interface{} `json:"items"`
	}
	if err := json.Unmarshal(bodyBytes, &dirListing); err == nil {
		// If it has an "items" key, we can safely assume it's a directory.
		return true, nil
	}

	return false, fmt.Errorf("could not determine if '%s' is a directory: unexpected JSON structure", remotePath)
}

func (c *Client) Download(remotePath, localPath string) {
	// Check if the provided localPath exists and is a directory
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		// It's a directory. Construct the new path to save the file inside it.
		baseName := filepath.Base(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if err != nil && !os.IsNotExist(err) {
		// It's some other error with the local path (e.g., permission denied)
		exitWithError("Error accessing local path %s: %v", localPath, err)
	}

	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		exitWithError("Error: %v", err)
	}

	// If downloading a directory, ensure .zip suffix
	if isDir && !strings.HasSuffix(localPath, ".zip") {
		localPath += ".zip"
	}

	var downloadURL string
	var headers map[string]string

	if isDir {
		fmt.Printf("Downloading directory '%s' as a zip file to '%s'...\n", remotePath, localPath)
		downloadURL = "/api/raw" + encodePathPreserveSlash(remotePath) + "?action=download&format=zip"
		headers = map[string]string{"Accept": "application/zip"}
	} else {
		fmt.Printf("Downloading file '%s' to '%s'...\n", remotePath, localPath)
		downloadURL = "/api/raw" + encodePathPreserveSlash(remotePath)
		headers = nil // Default headers
	}

	resp, err := c.apiRequest("GET", downloadURL, nil, headers)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("Download failed: %s", string(b))
	}

	out, err := os.Create(localPath)
	if err != nil {
		exitWithError("%v", err)
	}
	defer closeFileWithDebug(out, "Download")

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		exitWithError("Error saving downloaded file: %v", err)
	}
	fmt.Println("Download complete.")
}

// encodePathPreserveSlash encodes a path, preserving slashes (for read ops)
func encodePathPreserveSlash(p string) string {
	return (&url.URL{Path: p}).EscapedPath()
}

// encodeSegments encodes each segment for write operations (mkdir, upload, delete, rename)
func encodeSegments(p string) string {
	trimmed := strings.Trim(p, "/") // Only trim slashes, not whitespace
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return "/" + strings.Join(parts, "/")
}

func (c *Client) LsIgnoreScript(remotePath, ignoreName string, scriptMode bool) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("API error %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []struct {
			Name     string
			IsDir    bool
			Modified string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		exitWithError("Failed to decode response: %v", err)
	}
	// Deduplicate by normalized name, keep most recent Modified
	type entry struct {
		Name     string
		IsDir    bool
		Modified string
	}
	dedup := make(map[string]entry)
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		var err error
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	for _, item := range data.Items {
		norm := strings.TrimRight(item.Name, "\r\n")
		if strings.TrimSpace(norm) == "" {
			continue // skip blank/ghost entries after normalization
		}
		if ignoreRegex != nil && shouldIgnoreRegex(norm, ignoreRegex) {
			continue
		}
		if e, ok := dedup[norm]; !ok || item.Modified > e.Modified {
			dedup[norm] = entry{item.Name, item.IsDir, item.Modified}
		}
	}
	// Sort by Modified descending, then by Name
	sorted := make([]entry, 0, len(dedup))
	for _, v := range dedup {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Modified == sorted[j].Modified {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Modified > sorted[j].Modified
	})

	// Print output
	if scriptMode {
		// Script-friendly mode: one name per line, no colors, no formatting
		for _, e := range sorted {
			name := e.Name
			name = strings.ReplaceAll(name, "\n", "")
			name = strings.ReplaceAll(name, "\r", "")
			if e.IsDir && !strings.HasSuffix(name, "/") {
				name += "/"
			}
			fmt.Println(name)
		}
		return
	}

	// Human-friendly mode: multi-column with colors
	// Print like bash: tab-separated, trailing slash for dirs
	const (
		colorBlue  = "\033[1;34m"
		colorReset = "\033[0m"
	)
	// Prepare names and calculate max width
	names := make([]string, len(sorted))
	maxLen := 0
	for i, e := range sorted {
		name := e.Name
		name = strings.ReplaceAll(name, "\n", "")
		name = strings.ReplaceAll(name, "\r", "")
		if e.IsDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		if e.IsDir {
			name = colorBlue + name + colorReset
		}
		names[i] = name
		// Visible length (strip color codes for width)
		visible := len([]rune(stripANSICodes(name)))
		if visible > maxLen {
			maxLen = visible
		}
	}
	if maxLen < 16 {
		maxLen = 16
	}
	// Get terminal width (default 80)
	width := 80
	if w, ok := getTerminalWidth(); ok {
		width = w
	}
	colWidth := maxLen + 2
	cols := width / colWidth
	if cols < 1 {
		cols = 1
	}
	rows := (len(names) + cols - 1) / cols
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			i := c*rows + r
			if i >= len(names) {
				continue
			}
			name := names[i]
			fmt.Printf("%-*s", colWidth+len(name)-len(stripANSICodes(name)), name)
		}
		fmt.Println()
	}
}

// ListIgnore lists files/directories with detailed info, ignoring entries matching ignoreName
func (c *Client) ListIgnore(remotePath, ignoreName string) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		exitWithError("%v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing response body: %v\n", err)
		}
	}()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		exitWithError("API error %d: %s", resp.StatusCode, string(b))
	}
	var data struct {
		Items []struct {
			Name     string
			IsDir    bool
			Size     int64
			Modified string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		exitWithError("Failed to decode response: %v", err)
	}
	type entry struct {
		Name     string
		IsDir    bool
		Modified string
		Size     int64
	}
	dedup := make(map[string]entry)
	maxName := 4 // min width for 'Name'
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		var err error
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	for _, item := range data.Items {
		norm := strings.TrimRight(item.Name, "\r\n")
		if strings.TrimSpace(norm) == "" {
			continue // skip blank/ghost entries after normalization
		}
		if ignoreRegex != nil && shouldIgnoreRegex(norm, ignoreRegex) {
			continue
		}
		if e, ok := dedup[norm]; !ok || item.Modified > e.Modified {
			dedup[norm] = entry{item.Name, item.IsDir, item.Modified, item.Size}
		}
		if l := len(item.Name); l > maxName {
			maxName = l
		}
	}
	if maxName > 60 {
		maxName = 60
	} else if maxName < 30 {
		maxName = 30
	}
	sorted := make([]entry, 0, len(dedup))
	for _, v := range dedup {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Modified == sorted[j].Modified {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Modified > sorted[j].Modified
	})
	fmt.Printf("%-*s %-19s %-8s\n", maxName, "Name", "Modified", "Size")
	fmt.Printf("%-*s %-19s %-8s\n", maxName, strings.Repeat("-", maxName), strings.Repeat("-", 19), strings.Repeat("-", 8))
	const (
		colorBlue  = "\033[1;34m"
		colorReset = "\033[0m"
	)
	for _, e := range sorted {
		name := e.Name
		name = strings.ReplaceAll(name, "\n", "")
		name = strings.ReplaceAll(name, "\r", "")
		if e.IsDir && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		date := e.Modified
		if len(date) > 19 && strings.Contains(date, "T") {
			date = strings.Replace(date[:19], "T", " ", 1)
		} else if len(date) > 19 {
			date = date[:19]
		}
		visibleLen := len([]rune(name))
		if e.IsDir {
			colorName := colorBlue + name + colorReset
			pad := maxName - visibleLen
			if pad < 0 {
				pad = 0
			}
			fmt.Printf("%s%s %-19s %-8d\n", colorName, strings.Repeat(" ", pad), date, e.Size)
		} else {
			fmt.Printf("%-*s %-19s %-8d\n", maxName, name, date, e.Size)
		}
	}
}

// UploadIgnore is like Upload but skips files/dirs matching ignoreName (regex)
func (c *Client) UploadIgnore(localPath, remoteDir, ignoreName string) {
	info, err := os.Stat(localPath)
	if err != nil {
		exitWithError("Error accessing local path: %v", err)
	}
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	if !info.IsDir() {
		if ignoreRegex != nil && shouldIgnoreRegex(info.Name(), ignoreRegex) {
			fmt.Printf("Ignoring file: %s\n", info.Name())
			return
		}
		err := c.uploadFile(localPath, remoteDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		} else {
			fmt.Println("Upload complete.")
		}
		return
	}
	fmt.Printf("Uploading directory '%s' to '%s' (ignoring '%s')\n", localPath, remoteDir, ignoreName)
	localDirName := filepath.Base(localPath)
	if ignoreRegex != nil && shouldIgnoreRegex(localDirName, ignoreRegex) {
		fmt.Printf("Ignoring directory: %s\n", localDirName)
		return
	}
	fullRemoteDir := path.Join(remoteDir, localDirName)
	c.Mkdir(fullRemoteDir)
	walkErr := filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		if relPath == "." {
			return nil
		}
		if ignoreRegex != nil {
			pathParts := strings.Split(relPath, "/")
			for _, part := range pathParts {
				if shouldIgnoreRegex(part, ignoreRegex) {
					if info.IsDir() {

						return filepath.SkipDir
					}

					return nil
				}
			}
		}
		remoteItemPath := path.Join(fullRemoteDir, relPath)
		if info.IsDir() {
			fmt.Printf("Creating remote directory: %s\n", remoteItemPath)
			c.Mkdir(remoteItemPath)
		} else {
			remoteParentDir := path.Dir(remoteItemPath)
			fmt.Printf("Uploading file %s to %s\n", currentLocalPath, remoteParentDir)
			err := c.uploadFile(currentLocalPath, remoteParentDir)
			if err != nil {
				return fmt.Errorf("failed to upload %s: %w", currentLocalPath, err)
			}
		}
		return nil
	})
	if walkErr != nil {
		exitWithError("Error during directory upload: %v", walkErr)
	}
	fmt.Println("Directory upload complete.")
}

// DownloadIgnore is like Download but skips remote files/dirs matching ignoreName (regex) when downloading directories
func (c *Client) DownloadIgnore(remotePath, localPath, ignoreName string) {
	info, err := os.Stat(localPath)
	if err == nil && info.IsDir() {
		baseName := filepath.Base(remotePath)
		localPath = filepath.Join(localPath, baseName)
	} else if err != nil && !os.IsNotExist(err) {
		exitWithError("Error accessing local path %s: %v", localPath, err)
	}
	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		exitWithError("Error: %v", err)
	}
	var ignoreRegex *regexp.Regexp
	if ignoreName != "" {
		ignoreRegex, err = regexp.Compile(ignoreName)
		if err != nil {
			exitWithError("Invalid ignore regex: %v", err)
		}
	}
	if !isDir {
		if ignoreRegex != nil && shouldIgnoreRegex(path.Base(remotePath), ignoreRegex) {
			fmt.Printf("Ignoring file: %s\n", path.Base(remotePath))
			return
		}
		if err := c.downloadFile(remotePath, localPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
		}
		return
	}
	// Directory download with ignore
	fmt.Printf("Downloading directory '%s' as a zip file to '%s' (ignoring '%s')...\n", remotePath, localPath, ignoreName)
	// Instead of zip, recursively download, skipping ignored
	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		exitWithError("Failed to create directory %s: %v", localPath, err)
	}
	var downloadDir func(string, string)
	downloadDir = func(rPath, lPath string) {
		items, err := c.listRemote(rPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list remote directory %s: %v\n", rPath, err)
			return
		}
		for _, item := range items {
			if ignoreRegex != nil && shouldIgnoreRegex(item.Name, ignoreRegex) {

				continue
			}
			remoteItemPath := path.Join(rPath, item.Name)
			localItemPath := filepath.Join(lPath, item.Name)
			if item.IsDir {
				if err := os.MkdirAll(localItemPath, os.ModePerm); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create directory %s: %v\n", localItemPath, err)
					continue
				}
				downloadDir(remoteItemPath, localItemPath)
			} else {
				if err := c.downloadFile(remoteItemPath, localItemPath); err != nil {
					fmt.Fprintf(os.Stderr, "Failed to download file: %v\n", err)
				}
			}
		}
	}
	downloadDir(remotePath, localPath)
	fmt.Println("Directory download complete.")
}

// shouldIgnoreRegex returns true if name matches the ignore regex
func shouldIgnoreRegex(name string, ignoreRegex *regexp.Regexp) bool {
	return ignoreRegex.MatchString(name)
}
