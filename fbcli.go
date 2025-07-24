package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	// Removed syscall and unsafe imports
	"golang.org/x/term"
)

var version = "dev" // this will be set by the build process

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
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Error getting credentials: %v\n", err)
		os.Exit(1)
	}

	if cmd == "show" {
		client.ShowConfig()
		os.Exit(0)
	}

	if err := client.Login(); err != nil {
		fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
		os.Exit(1)
	}

	// Commands that take a single path argument
	singlePathCommands := map[string]func(string){
		"mkdir":  client.Mkdir,
		"rm":     client.Delete,
		"delete": client.Delete,
	}

	// Commands that take two path arguments
	twoPathCommands := map[string]func(string, string){
		"rename": client.Rename,
		"mv":     client.Rename,
	}

	if cmd == "ls" || cmd == "list" {
		remotePath := "/"
		if len(args) > 0 {
			remotePath = strings.Join(args, " ")
		}
		if cmd == "ls" {
			client.Ls(remotePath)
		} else {
			client.List(remotePath)
		}
	} else if fn, ok := singlePathCommands[cmd]; ok {
		if len(args) < 1 {
			usage()
			os.Exit(1)
		}
		fn(strings.Join(args, " "))
	} else if cmd == "upload" {
		if len(args) < 1 || len(args) > 2 {
			usage()
			os.Exit(1)
		}
		remotePath := "/"
		if len(args) == 2 {
			remotePath = args[1]
		}
		client.Upload(args[0], remotePath)
	} else if fn, ok := twoPathCommands[cmd]; ok {
		if len(args) != 2 {
			usage()
			os.Exit(1)
		}
		fn(args[0], args[1])
	} else if cmd == "download" { // Special handling for download to allow optional localPath
		if len(args) < 1 || len(args) > 2 {
			usage()
			os.Exit(1)
		}
		remotePath := args[0]
		localPath := ""
		if len(args) == 2 {
			localPath = args[1]
		} else {
			// If localPath is not provided, use the base name of the remotePath
			localPath = filepath.Base(remotePath)
		}
		client.Download(remotePath, localPath)
	} else {
		usage()
		os.Exit(1)
	}
}

func (c *Client) getCredentials() error {
	reader := bufio.NewReader(os.Stdin)

	if c.Config.URL == "" {
		fmt.Print("Enter File Browser URL: ")
		url, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		c.Config.URL = strings.TrimSpace(url)
	}

	if c.Config.Username == "" {
		fmt.Print("Enter Username: ")
		username, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		c.Config.Username = strings.TrimSpace(username)
	}

	if c.Config.Password == "" {
		fmt.Print("Enter Password: ")
		// Get the file descriptor for stdin
		fd := int(os.Stdin.Fd())

		// Check if stdin is a terminal
		if !term.IsTerminal(fd) {
			// If not a terminal, just read the line as is (e.g., from a pipe)
			password, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			c.Config.Password = strings.TrimSpace(password)
			return nil
		}

		// It's a terminal, so handle masked input
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			return err
		}
		defer term.Restore(fd, oldState)

		var password []byte
		var backspace = []byte{'\b', ' ', '\b'} // sequence to erase a character

		for {
			var buf [1]byte
			n, err := os.Stdin.Read(buf[:])
			if err != nil || n == 0 {
				return err
			}

			char := buf[0]

			// Enter key
			if char == '\r' || char == '\n' {
				fmt.Print("\r\n")
				break
			}

			// Backspace/Delete key (ASCII backspace is 8, delete is 127)
			if char == 8 || char == 127 {
				if len(password) > 0 {
					password = password[:len(password)-1]
					os.Stdout.Write(backspace)
				}
			} else if char == 3 { // Ctrl+C
				return fmt.Errorf("interrupted")
			} else {
				password = append(password, char)
				fmt.Print("*")
			}
		}
		c.Config.Password = string(password)
	}

	return nil
}

func usage() {
	fmt.Printf("goclient version %s\n", version)
	fmt.Println(`Usage: goclient <command> [arguments...]
Commands:
  ls [remote_path]                List file/directory names (optional remote_path)
  list [remote_path]              List detailed info (like ls -la) (optional remote_path)
  upload <local_path> [remote_dir]    Upload a file or directory (optional remote_dir)
  download <remote_path> [local_path] Download a file or directory (optional local_path)
  mkdir <remote_path>                 Create a directory
  rm <remote_path>                    Delete a file or directory
  rename <old_path> <new_path>        Rename a file or directory
  show                              Show the current configuration
`)
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	if c.Token != "" {
		req.AddCookie(&http.Cookie{Name: "auth", Value: c.Token})
		req.Header.Set("X-Auth", c.Token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

func (c *Client) Ls(remotePath string) {
	resp, err := c.apiRequest("GET", "/api/resources"+encodePathPreserveSlash(remotePath), nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "API error %d: %s", resp.StatusCode, string(b))
		return
	}
	var data struct {
		Items []struct {
			Name     string
			IsDir    bool
			Modified string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Fprintln(os.Stderr, "Failed to decode response:", err)
		return
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
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "API error %d: %s", resp.StatusCode, string(b))
		return
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
		fmt.Fprintln(os.Stderr, "Failed to decode response:", err)
		return
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
	fmt.Printf("%-*s %-19s %-8s\n", maxName, "Name", "Modified", "Size")
	fmt.Printf("%-*s %-19s %-8s\n", maxName, strings.Repeat("-", maxName), strings.Repeat("-", 19), strings.Repeat("-", 8))
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
			fmt.Printf("%s%s %-19s %-8d\n", colorName, strings.Repeat(" ", maxName-visibleLen), date, e.Size)
		} else {
			fmt.Printf("%-*s %-19s %-8d\n", maxName, name, date, e.Size)
		}
	}
}

func (c *Client) Mkdir(remotePath string) {
	cleanPath := strings.TrimSpace(remotePath)
	if cleanPath == "/" || cleanPath == "" {
		fmt.Fprintln(os.Stderr, "Cannot create root directory.")
		return
	}
	trimmed := strings.Trim(cleanPath, "/")
	if trimmed == "" {
		fmt.Fprintln(os.Stderr, "Invalid directory name.")
		return
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
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 409 { // 409 = already exists
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Directory creation failed. Server responded with HTTP %d.\n%s\n", resp.StatusCode, string(b))
		return
	}
	fmt.Println("Directory created.")
}

func (c *Client) Delete(remotePath string) {
	encoded := encodePathPreserveSlash(remotePath)
	if encoded == "" {
		fmt.Fprintln(os.Stderr, "Invalid path.")
		return
	}

	resp, err := c.apiRequest("DELETE", "/api/resources"+encoded, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Delete failed: %s\n", string(b))
		return
	}
	fmt.Println("Deletion complete.")
}

func (c *Client) Rename(oldPath, newPath string) {
	oldP := encodePathPreserveSlash(oldPath)
	newP := encodePathPreserveSlash(newPath)
	url := fmt.Sprintf("/api/resources%s?action=rename&destination=%s&override=false&rename=false", oldP, newP)
	resp, err := c.apiRequest("PATCH", url, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Rename failed: %s\n", string(b))
		return
	}
	fmt.Println("Rename complete.")
}

func (c *Client) Upload(localPath, remoteDir string) {
	info, err := os.Stat(localPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error accessing local path:", err)
		return
	}

	if !info.IsDir() {
		err := c.uploadFile(localPath, remoteDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		} else {
			fmt.Println("Upload complete.")
		}
		return
	}

	fmt.Printf("Uploading directory '%s' to '%s'\n", localPath, remoteDir)

	localDirName := filepath.Base(localPath)
	fullRemoteDir := path.Join(remoteDir, localDirName)

	c.Mkdir(fullRemoteDir)

	err = filepath.Walk(localPath, func(currentLocalPath string, info os.FileInfo, err error) error {
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

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during directory upload: %v\n", err)
		return
	}

	fmt.Println("Directory upload complete.")
}

func (c *Client) uploadFile(localPath, remoteDir string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	filename := filepath.Base(localPath)
	remoteDirEnc := encodePathPreserveSlash(remoteDir)
	filenameEnc := encodePathPreserveSlash(filename)
	url := fmt.Sprintf("/api/resources%s/%s?override=true", remoteDirEnc, filenameEnc)
	headers := map[string]string{"Content-Type": "application/octet-stream"}
	resp, err := c.apiRequest("POST", url, file, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		// Try to create directory, then retry upload
		c.Mkdir(remoteDir)
		file.Seek(0, io.SeekStart)
		resp, err = c.apiRequest("POST", url, file, headers)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server response %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *Client) isRemotePathDir(remotePath string) (bool, error) {
	path := encodePathPreserveSlash(remotePath)
	resp, err := c.apiRequest("GET", "/api/resources"+path, nil, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

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
		fmt.Fprintf(os.Stderr, "Error accessing local path %s: %v\n", localPath, err)
		return
	}

	isDir, err := c.isRemotePathDir(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
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
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "Download failed: %s\n", string(b))
		return
	}

	out, err := os.Create(localPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error saving downloaded file:", err)
		return
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
