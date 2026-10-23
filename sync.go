package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

// SyncTo makes the remote directory mirror the local directory. Files and directories
// present locally are copied; remote-only entries are deleted. Matched names are left
// untouched on both sides.
func (c *Client) SyncTo(localPath, remotePath, ignorePattern string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("access local path %q: %w", localPath, err)
	}
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	remotePath = normalizeRemotePath(remotePath)
	if !info.IsDir() {
		if ignore != nil && ignore.MatchString(info.Name()) {
			fmt.Printf("Ignoring file: %s\n", info.Name())
			return nil
		}
		return c.syncFileToRemote(localPath, path.Join(remotePath, info.Name()), info)
	}

	if ignore != nil && ignore.MatchString(info.Name()) {
		fmt.Printf("Ignoring directory: %s\n", info.Name())
		return nil
	}

	if ignorePattern == "" {
		fmt.Printf("Syncing from local '%s' to remote '%s'\n", localPath, remotePath)
	} else {
		fmt.Printf("Syncing from local '%s' to remote '%s' (ignoring '%s')\n", localPath, remotePath, ignorePattern)
	}

	if err := c.ensureRemoteDir(remotePath); err != nil {
		return err
	}
	return c.syncDirToRemote(localPath, remotePath, ignore)
}

func (c *Client) syncDirToRemote(localDir, remoteDir string, ignore *regexp.Regexp) error {
	localEntries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read local directory %s: %w", localDir, err)
	}

	remoteItems, err := c.listRemote(remoteDir)
	if err != nil {
		if !isNotFound(err) {
			return err
		}
		remoteItems = nil
	}

	remoteByName := make(map[string]Resource, len(remoteItems))
	for _, item := range remoteItems {
		remoteByName[item.Name] = item
	}

	for _, entry := range localEntries {
		name := entry.Name()
		if ignore != nil && ignore.MatchString(name) {
			continue
		}

		localItemPath := filepath.Join(localDir, name)
		remoteItemPath := path.Join(remoteDir, name)
		remoteItem, exists := remoteByName[name]
		delete(remoteByName, name)

		if entry.IsDir() {
			if exists && !remoteItem.IsDir {
				if err := c.removeRemote(remoteItemPath); err != nil {
					return err
				}
			}
			fmt.Printf("Creating remote directory: %s\n", remoteItemPath)
			if err := c.ensureRemoteDir(remoteItemPath); err != nil {
				return err
			}
			if err := c.syncDirToRemote(localItemPath, remoteItemPath, ignore); err != nil {
				return err
			}
			continue
		}

		if exists && remoteItem.IsDir {
			if err := c.removeRemote(remoteItemPath); err != nil {
				return err
			}
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat local file %s: %w", localItemPath, err)
		}
		if err := c.syncFileToRemote(localItemPath, remoteItemPath, info); err != nil {
			return err
		}
	}

	// Anything left on the remote side does not exist locally.
	for name := range remoteByName {
		if ignore != nil && ignore.MatchString(name) {
			continue
		}
		remoteItemPath := path.Join(remoteDir, name)
		fmt.Printf("Deleting remote entry not in source: %s\n", remoteItemPath)
		if err := c.removeRemote(remoteItemPath); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) syncFileToRemote(localPath, remotePath string, localInfo os.FileInfo) error {
	remoteItem, err := c.getResource(remotePath)
	if err != nil {
		if !isNotFound(err) {
			return err
		}
		return c.uploadFileToPath(localPath, remotePath)
	}

	if remoteItem.IsDir {
		if err := c.removeRemote(remotePath); err != nil {
			return err
		}
		return c.uploadFileToPath(localPath, remotePath)
	}

	remoteSize, err := c.getExactFileSize(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read size of %s: %v\n", remotePath, err)
		return c.uploadFileToPath(localPath, remotePath)
	}
	if remoteSize != localInfo.Size() {
		fmt.Printf("File size mismatch for %s. Uploading.\n", localPath)
		return c.uploadFileToPath(localPath, remotePath)
	}

	if localInfo.Size() >= hashThreshold {
		fmt.Printf("File %s is already in sync (size match, not hashing >1MB).\n", localPath)
		return nil
	}

	localHash, err := getLocalFileHash(localPath)
	if err != nil {
		return fmt.Errorf("hash local file %s: %w", localPath, err)
	}
	remoteHash, err := c.getChecksum(remotePath, "sha256")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read checksum of %s: %v\n", remotePath, err)
		fmt.Printf("File hash unknown for %s. Uploading.\n", localPath)
		return c.uploadFileToPath(localPath, remotePath)
	}
	if localHash != remoteHash {
		fmt.Printf("File hash mismatch for %s. Uploading.\n", localPath)
		return c.uploadFileToPath(localPath, remotePath)
	}

	fmt.Printf("File %s is already in sync.\n", localPath)
	return nil
}

// SyncFrom makes the local directory mirror the remote directory.
func (c *Client) SyncFrom(remotePath, localPath, ignorePattern string) error {
	remotePath = normalizeRemotePath(remotePath)
	ignore, err := compileIgnore(ignorePattern)
	if err != nil {
		return err
	}

	item, err := c.getResource(remotePath)
	if err != nil {
		return err
	}

	if !item.IsDir {
		if ignore != nil && ignore.MatchString(path.Base(remotePath)) {
			fmt.Printf("Ignoring file: %s\n", path.Base(remotePath))
			return nil
		}
		localPath = resolveLocalFileTarget(remotePath, localPath)
		return c.syncFileFromRemote(remotePath, localPath)
	}

	if ignorePattern == "" {
		fmt.Printf("Syncing remote directory '%s' to local '%s'\n", remotePath, localPath)
	} else {
		fmt.Printf("Syncing remote directory '%s' to local '%s' (ignoring '%s')\n", remotePath, localPath, ignorePattern)
	}

	if err := os.MkdirAll(localPath, os.ModePerm); err != nil {
		return fmt.Errorf("create local directory %s: %w", localPath, err)
	}
	return c.syncDirFromRemote(remotePath, localPath, ignore)
}

func (c *Client) syncDirFromRemote(remoteDir, localDir string, ignore *regexp.Regexp) error {
	remoteItems, err := c.listRemote(remoteDir)
	if err != nil {
		return err
	}

	localEntries, err := os.ReadDir(localDir)
	if err != nil {
		return fmt.Errorf("read local directory %s: %w", localDir, err)
	}
	localByName := make(map[string]os.DirEntry, len(localEntries))
	for _, entry := range localEntries {
		localByName[entry.Name()] = entry
	}

	for _, item := range remoteItems {
		name := item.Name
		if ignore != nil && ignore.MatchString(name) {
			continue
		}

		remoteItemPath := path.Join(remoteDir, name)
		localItemPath := filepath.Join(localDir, name)
		localEntry, exists := localByName[name]
		delete(localByName, name)

		if item.IsDir {
			if exists && !localEntry.IsDir() {
				if err := os.RemoveAll(localItemPath); err != nil {
					return err
				}
			}
			if err := os.MkdirAll(localItemPath, os.ModePerm); err != nil {
				return fmt.Errorf("create local directory %s: %w", localItemPath, err)
			}
			if err := c.syncDirFromRemote(remoteItemPath, localItemPath, ignore); err != nil {
				return err
			}
			continue
		}

		if exists && localEntry.IsDir() {
			if err := os.RemoveAll(localItemPath); err != nil {
				return err
			}
		}
		if err := c.syncFileFromRemoteItem(remoteItemPath, localItemPath); err != nil {
			return err
		}
	}

	// Anything left locally does not exist on the remote side.
	for name, entry := range localByName {
		if ignore != nil && ignore.MatchString(name) {
			continue
		}
		localItemPath := filepath.Join(localDir, name)
		fmt.Printf("Deleting local entry not in remote: %s\n", localItemPath)
		if entry.IsDir() {
			if err := os.RemoveAll(localItemPath); err != nil {
				return fmt.Errorf("delete local directory %s: %w", localItemPath, err)
			}
			continue
		}
		if err := os.Remove(localItemPath); err != nil {
			return fmt.Errorf("delete local file %s: %w", localItemPath, err)
		}
	}
	return nil
}

func (c *Client) syncFileFromRemote(remotePath, localPath string) error {
	if _, err := c.getResource(remotePath); err != nil {
		return err
	}
	return c.syncFileFromRemoteItem(remotePath, localPath)
}

func (c *Client) syncFileFromRemoteItem(remotePath, localPath string) error {
	localInfo, err := os.Stat(localPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat local file %s: %w", localPath, err)
		}
		return c.downloadFile(remotePath, localPath)
	}
	if localInfo.IsDir() {
		if err := os.RemoveAll(localPath); err != nil {
			return err
		}
		return c.downloadFile(remotePath, localPath)
	}

	remoteSize, err := c.getExactFileSize(remotePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read size of %s: %v\n", remotePath, err)
		return c.downloadFile(remotePath, localPath)
	}
	if remoteSize != localInfo.Size() {
		fmt.Printf("File size mismatch for %s. Downloading.\n", remotePath)
		return c.downloadFile(remotePath, localPath)
	}

	if localInfo.Size() >= hashThreshold {
		fmt.Printf("File %s is already in sync (size match, not hashing >1MB).\n", localPath)
		return nil
	}

	localHash, err := getLocalFileHash(localPath)
	if err != nil {
		return fmt.Errorf("hash local file %s: %w", localPath, err)
	}
	remoteHash, err := c.getChecksum(remotePath, "sha256")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not read checksum of %s: %v\n", remotePath, err)
		fmt.Printf("File hash unknown for %s. Downloading.\n", remotePath)
		return c.downloadFile(remotePath, localPath)
	}
	if localHash != remoteHash {
		fmt.Printf("File hash mismatch for %s. Downloading.\n", remotePath)
		return c.downloadFile(remotePath, localPath)
	}

	fmt.Printf("File %s is already in sync.\n", localPath)
	return nil
}

// getLocalFileHash returns the lowercase hex SHA-256 digest of a local file.
func getLocalFileHash(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer closeFile(file)
	return hashReader(file)
}
