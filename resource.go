package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	hashThreshold = 1024 * 1024 // Files below this size are verified with sha256.
	colorBlue     = "\033[1;34m"
	colorReset    = "\033[0m"
)

// List prints a remote directory (or a single file) in one of the supported formats.
func (c *Client) List(remotePath, ignorePattern string, long, script bool) error {
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	item, err := c.getResource(remotePath)
	if err != nil {
		return err
	}

	var items []Resource
	if item.IsDir {
		items, err = c.listRemote(remotePath)
		if err != nil {
			return err
		}
	} else {
		items = []Resource{*item}
	}

	filtered := items[:0]
	for _, entry := range items {
		if ignore != nil && ignore.MatchString(entry.Name) {
			continue
		}
		filtered = append(filtered, entry)
	}
	sortResources(filtered)

	if long {
		printLongList(filtered)
	} else {
		printShortList(filtered, script)
	}
	return nil
}

// Mkdir creates a directory (including missing parents) and is idempotent.
func (c *Client) Mkdir(remotePath string) error {
	remotePath = normalizeRemotePath(remotePath)
	if remotePath == "/" {
		return fmt.Errorf("cannot create the root directory")
	}
	if err := c.mkdirRemote(remotePath); err != nil {
		return err
	}
	fmt.Printf("Directory created: %s\n", remotePath)
	return nil
}

// mkdirRemote creates a directory without printing anything. It is idempotent for an
// existing directory but never replaces an existing file.
func (c *Client) mkdirRemote(remotePath string) error {
	remotePath = normalizeRemotePath(remotePath)
	query := url.Values{}
	query.Set("path", remotePath)
	query.Set("isDir", "true")

	resp, err := c.do(http.MethodPost, "/api/resources", query, nil, nil)
	if err != nil {
		return err
	}
	defer closeResponse(resp)

	if resp.StatusCode == http.StatusConflict {
		item, lookupErr := c.getResource(remotePath)
		if lookupErr == nil {
			if item.IsDir {
				return nil
			}
			return fmt.Errorf("path %s already exists and is not a directory", remotePath)
		}
		return responseError(resp)
	}
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

// Delete removes a file or directory. Directories are removed recursively by the server.
func (c *Client) Delete(remotePath string) error {
	if err := c.removeRemote(remotePath); err != nil {
		return err
	}
	fmt.Println("Deletion complete.")
	return nil
}

// DeleteIgnore removes a path while preserving children whose names match the pattern.
func (c *Client) DeleteIgnore(remotePath, ignorePattern string) error {
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}
	if ignore == nil {
		return c.Delete(remotePath)
	}

	remotePath = normalizeRemotePath(remotePath)
	if remotePath == "/" {
		return fmt.Errorf("refusing to delete the source root directory")
	}
	item, err := c.getResource(remotePath)
	if err != nil {
		return err
	}

	if !item.IsDir {
		if ignore.MatchString(path.Base(remotePath)) {
			fmt.Printf("Ignoring file: %s\n", remotePath)
			return nil
		}
		if err := c.removeRemote(remotePath); err != nil {
			return err
		}
		fmt.Println("Deletion complete.")
		return nil
	}

	fmt.Printf("Deleting contents of '%s' (ignoring '%s')\n", remotePath, ignorePattern)
	kept, err := c.deleteTree(remotePath, ignore)
	if err != nil {
		return err
	}
	if !kept {
		if err := c.removeRemote(remotePath); err != nil {
			return err
		}
	}
	fmt.Println("Deletion complete.")
	return nil
}

// deleteTree removes the children of a directory and reports whether anything was kept.
func (c *Client) deleteTree(remoteDir string, ignore *regexp.Regexp) (bool, error) {
	items, err := c.listRemote(remoteDir)
	if err != nil {
		return false, err
	}

	kept := false
	for _, item := range items {
		itemPath := path.Join(remoteDir, item.Name)
		if ignore != nil && ignore.MatchString(item.Name) {
			fmt.Printf("Ignoring: %s\n", itemPath)
			kept = true
			continue
		}
		if item.IsDir {
			childKept, err := c.deleteTree(itemPath, ignore)
			if err != nil {
				return kept || childKept, err
			}
			if childKept {
				kept = true
				continue
			}
		}
		if err := c.removeRemote(itemPath); err != nil {
			return kept, err
		}
	}
	return kept, nil
}

// removeRemote deletes a remote path without printing a success message.
func (c *Client) removeRemote(remotePath string) error {
	remotePath = normalizeRemotePath(remotePath)
	if remotePath == "/" {
		return fmt.Errorf("refusing to delete the source root directory")
	}

	query := url.Values{}
	query.Set("path", remotePath)

	resp, err := c.do(http.MethodDelete, "/api/resources", query, nil, nil)
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return responseError(resp)
	}
	return nil
}

// Rename moves or renames a resource using the v2 JSON PATCH endpoint.
func (c *Client) Rename(oldPath, newPath string) error {
	request := map[string]interface{}{
		"items": []map[string]string{{
			"fromSource": c.Source,
			"fromPath":   normalizeRemotePath(oldPath),
			"toSource":   c.Source,
			"toPath":     normalizeRemotePath(newPath),
		}},
		"action":    "rename",
		"overwrite": false,
		"rename":    false,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}

	resp, err := c.do(http.MethodPatch, "/api/resources", nil, bytes.NewReader(body),
		map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return err
	}
	defer closeResponse(resp)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		return responseError(resp)
	}

	var result struct {
		Failed []struct {
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"failed"`
	}
	if resp.StatusCode == http.StatusMultiStatus {
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("rename partially failed (HTTP %d)", resp.StatusCode)
		}
	}
	if len(result.Failed) > 0 {
		message := result.Failed[0].Message
		if message == "" {
			message = "operation failed"
		}
		return fmt.Errorf("rename failed: %s", message)
	}
	if resp.StatusCode == http.StatusMultiStatus {
		return fmt.Errorf("rename failed: server reported a partial failure")
	}

	fmt.Println("Rename complete.")
	return nil
}

// Upload copies a local file or directory tree to a remote directory.
func (c *Client) Upload(localPath, remoteDir, ignorePattern string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("access local path %q: %w", localPath, err)
	}
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	remoteDir = normalizeRemotePath(remoteDir)

	if !info.IsDir() {
		if ignore != nil && ignore.MatchString(info.Name()) {
			fmt.Printf("Ignoring file: %s\n", info.Name())
			return nil
		}

		target := path.Join(remoteDir, info.Name())
		// If the destination already exists as a file, treat it as the target path
		// (the same convention as cp/scp). Otherwise the destination is a directory.
		if remoteDir != "/" {
			existing, err := c.getResource(remoteDir)
			if err == nil && !existing.IsDir {
				target = remoteDir
			} else if err != nil && !isNotFound(err) {
				return err
			}
		}
		if err := c.uploadFileToPath(localPath, target); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
		fmt.Println("Upload complete.")
		return nil
	}

	if ignore != nil && ignore.MatchString(info.Name()) {
		fmt.Printf("Ignoring directory: %s\n", info.Name())
		return nil
	}

	remoteBase := path.Join(remoteDir, filepath.Base(localPath))
	fmt.Printf("Uploading directory '%s' to '%s'\n", localPath, remoteDir)
	if remoteDir != "/" {
		existing, err := c.getResource(remoteDir)
		if err == nil && !existing.IsDir {
			return fmt.Errorf("remote destination %s is a file, not a directory", remoteDir)
		}
		if err != nil && !isNotFound(err) {
			return err
		}
	}
	if err := c.ensureRemoteDir(remoteBase); err != nil {
		return err
	}

	err = filepath.Walk(localPath, func(currentLocalPath string, entry os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(localPath, currentLocalPath)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		if ignore != nil && ignore.MatchString(entry.Name()) {
			if entry.IsDir() {
				fmt.Printf("Ignoring directory: %s\n", relPath)
				return filepath.SkipDir
			}
			fmt.Printf("Ignoring file: %s\n", relPath)
			return nil
		}

		remoteItemPath := path.Join(remoteBase, relPath)
		if entry.IsDir() {
			return c.ensureRemoteDir(remoteItemPath)
		}
		if err := c.uploadFileToPath(currentLocalPath, remoteItemPath); err != nil {
			return fmt.Errorf("upload %s: %w", currentLocalPath, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("directory upload failed: %w", err)
	}

	fmt.Println("Directory upload complete.")
	return nil
}

// uploadFileToPath uploads a local file to an exact remote path.
func (c *Client) uploadFileToPath(localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer closeFile(file)

	query := url.Values{}
	query.Set("path", normalizeRemotePath(remotePath))
	query.Set("override", "true")

	fmt.Printf("Uploading file %s to %s\n", localPath, normalizeRemotePath(remotePath))
	resp, err := c.do(http.MethodPost, "/api/resources", query, file,
		map[string]string{"Content-Type": "application/octet-stream"})
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// ensureRemoteDir creates a remote directory if needed, replacing a file with the same name.
func (c *Client) ensureRemoteDir(remotePath string) error {
	remotePath = normalizeRemotePath(remotePath)
	item, err := c.getResource(remotePath)
	if err == nil {
		if item.IsDir {
			return nil
		}
		if err := c.removeRemote(remotePath); err != nil {
			return err
		}
		return c.mkdirRemote(remotePath)
	}
	if !isNotFound(err) {
		return err
	}
	return c.mkdirRemote(remotePath)
}

// Download copies a remote file or directory to the local filesystem.
func (c *Client) Download(remotePath, localPath, ignorePattern string, forceZip bool) error {
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	remotePath = normalizeRemotePath(remotePath)
	item, err := c.getResource(remotePath)
	if err != nil {
		return err
	}

	if !item.IsDir {
		if ignore != nil && ignore.MatchString(path.Base(remotePath)) {
			fmt.Printf("Ignoring file: %s\n", remotePath)
			return nil
		}
		target := resolveLocalFileTarget(remotePath, localPath)
		if err := c.downloadFile(remotePath, target); err != nil {
			return err
		}
		fmt.Println("Download complete.")
		return nil
	}

	if ignore != nil {
		if forceZip {
			return fmt.Errorf("-z (zip) and -i (ignore) cannot be used together")
		}
		target := resolveLocalDirTarget(remotePath, localPath)
		if err := os.MkdirAll(target, os.ModePerm); err != nil {
			return fmt.Errorf("create local directory %s: %w", target, err)
		}
		fmt.Printf("Downloading directory '%s' to '%s' (ignoring '%s')...\n", remotePath, target, ignorePattern)
		if err := c.downloadTree(remotePath, target, ignore); err != nil {
			return err
		}
		fmt.Println("Directory download complete.")
		return nil
	}

	target := resolveZipTarget(remotePath, localPath)
	if err := c.downloadArchive(remotePath, target); err != nil {
		return err
	}
	fmt.Println("Download complete.")
	return nil
}

func (c *Client) downloadTree(remoteDir, localDir string, ignore *regexp.Regexp) error {
	items, err := c.listRemote(remoteDir)
	if err != nil {
		return err
	}
	for _, item := range items {
		if ignore != nil && ignore.MatchString(item.Name) {
			continue
		}
		remoteItemPath := path.Join(remoteDir, item.Name)
		localItemPath := filepath.Join(localDir, item.Name)
		if item.IsDir {
			if err := os.MkdirAll(localItemPath, os.ModePerm); err != nil {
				return fmt.Errorf("create local directory %s: %w", localItemPath, err)
			}
			if err := c.downloadTree(remoteItemPath, localItemPath, ignore); err != nil {
				return err
			}
			continue
		}
		if err := c.downloadFile(remoteItemPath, localItemPath); err != nil {
			return err
		}
	}
	return nil
}

// downloadFile downloads a single file to an exact local path.
func (c *Client) downloadFile(remotePath, localPath string) error {
	fmt.Printf("Downloading file '%s' to '%s'\n", remotePath, localPath)

	query := url.Values{}
	query.Set("file", normalizeRemotePath(remotePath))
	resp, err := c.do(http.MethodGet, "/api/resources/download", query, nil, nil)
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return writeResponseToFile(resp.Body, localPath)
}

// downloadArchive asks the server to zip a directory and saves the stream locally.
func (c *Client) downloadArchive(remotePath, localPath string) error {
	fmt.Printf("Downloading directory '%s' as a zip file to '%s'...\n", remotePath, localPath)

	query := url.Values{}
	query.Set("file", normalizeRemotePath(remotePath))
	query.Set("algo", "zip")
	resp, err := c.do(http.MethodGet, "/api/resources/download", query, nil,
		map[string]string{"Accept": "application/zip"})
	if err != nil {
		return err
	}
	defer closeResponse(resp)
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return writeResponseToFile(resp.Body, localPath)
}

func writeResponseToFile(reader io.Reader, localPath string) error {
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		return fmt.Errorf("local path %s is a directory, not a file", localPath)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), os.ModePerm); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", localPath, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(localPath), ".fbcli-download-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, reader); err != nil {
		return fmt.Errorf("save download: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// CreateTemp creates 0600; use conventional read permissions for downloaded files.
	_ = os.Chmod(tmpName, 0o644)
	if err := os.Rename(tmpName, localPath); err != nil {
		return err
	}
	return nil
}

// downloadRecursive downloads a file or directory tree without creating an archive.
// It is used by the interactive shell so that "get" behaves like sftp.
func (c *Client) downloadRecursive(remotePath, localPath, ignorePattern string) error {
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	remotePath = normalizeRemotePath(remotePath)
	item, err := c.getResource(remotePath)
	if err != nil {
		return err
	}

	if !item.IsDir {
		if ignore != nil && ignore.MatchString(path.Base(remotePath)) {
			fmt.Printf("Ignoring file: %s\n", remotePath)
			return nil
		}
		target := resolveLocalFileTarget(remotePath, localPath)
		if err := c.downloadFile(remotePath, target); err != nil {
			return err
		}
		fmt.Println("Download complete.")
		return nil
	}

	target := localPath
	if target == "" {
		target = path.Base(remotePath)
	} else if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
		target = filepath.Join(target, path.Base(remotePath))
	}
	if err := os.MkdirAll(target, os.ModePerm); err != nil {
		return fmt.Errorf("create local directory %s: %w", target, err)
	}

	fmt.Printf("Downloading directory '%s' to '%s'...\n", remotePath, target)
	if err := c.downloadTree(remotePath, target, ignore); err != nil {
		return err
	}
	fmt.Println("Directory download complete.")
	return nil
}

func resolveLocalFileTarget(remotePath, localPath string) string {
	base := path.Base(remotePath)
	if localPath == "" {
		return base
	}
	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		return filepath.Join(localPath, base)
	}
	return localPath
}

func resolveLocalDirTarget(remotePath, localPath string) string {
	base := path.Base(remotePath)
	if localPath == "" {
		return base
	}
	return localPath
}

func resolveZipTarget(remotePath, localPath string) string {
	remoteBase := path.Base(remotePath)
	if localPath == "" {
		return remoteBase + ".zip"
	}

	if info, err := os.Stat(localPath); err == nil && info.IsDir() {
		return uniqueZipPath(filepath.Join(localPath, remoteBase))
	}

	if !strings.HasSuffix(strings.ToLower(localPath), ".zip") {
		localPath += ".zip"
	}
	zipBase := localPath[:len(localPath)-len(".zip")]
	return uniqueZipPath(zipBase)
}

// uniqueZipPath returns base+".zip", adding a numeric suffix while the candidate is a directory.
func uniqueZipPath(base string) string {
	candidate := base + ".zip"
	for i := 1; ; i++ {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d.zip", base, i)
	}
}

func compileIgnore(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid ignore regex %q: %w", pattern, err)
	}
	return re, nil
}

func closeFile(file *os.File) {
	if file != nil {
		_ = file.Close()
	}
}
