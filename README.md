# fbcli - FileBrowser Command Line Interface

A powerful command-line client for [FileBrowser](https://filebrowser.org), enabling efficient file management operations on remote FileBrowser instances from your terminal.

[![Go Version](https://img.shields.io/badge/Go-1.24.4-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](#license)

## 🚀 Features

- **Complete File Operations**: Upload, download, list, create, delete, rename files and directories
- **Advanced Listing**: Multiple listing modes including detailed view and script-friendly output
- **Sync Capabilities**: Bidirectional sync between local and remote directories
- **Pattern Filtering**: Regex-based ignore patterns for selective operations
- **Comprehensive Aliases**: Multiple command aliases for improved usability
- **Zip Downloads**: Automatic zip compression for directory downloads
- **Interactive Configuration**: Secure credential input with hidden password entry
- **Clean Test Suite**: 100% test coverage with automated cleanup

## 📦 Installation

### Pre-built Binaries
Download the latest release from the [releases page](https://github.com/johnwmail/fbcli/releases).

### Build from Source
```bash
git clone https://github.com/johnwmail/fbcli.git
cd fbcli
go build -o fbcli
```

### Using Go Install
```bash
go install github.com/johnwmail/fbcli@latest
```

## ⚙️ Configuration

fbcli requires three environment variables to connect to your FileBrowser instance:

```bash
export FILEBROWSER_URL="https://your-filebrowser-instance.com"
export FILEBROWSER_USERNAME="your-username"
export FILEBROWSER_PASSWORD="your-password"
```

Alternatively, fbcli will prompt for missing credentials interactively.

## 📋 Commands

### File Listing

#### `ls [-i ignore] [-l] [-s] [remote_path]`
List files and directories with multiple output formats.

**Flags:**
- `-l`: Detailed view with file sizes, dates, and permissions
- `-s`: Script-friendly output (one file per line, no colors)
- `-i <regex>`: Ignore files/directories matching the regex pattern

**Aliases:** `list`, `dir` (default to detailed view)

**Examples:**
```bash
# Basic listing
fbcli ls /

# Detailed listing with sizes and dates
fbcli ls -l /documents

# Script-friendly output for automation
fbcli ls -s /logs

# Ignore log files
fbcli ls -i ".*\\.log$" /var

# Combined flags
fbcli ls -l -i "temp.*" /workspace
```

### File Transfer

#### `upload, up [-i ignore] <local_path> [remote_dir]`
Upload files or directories to the remote server.

**Examples:**
```bash
# Upload file to root
fbcli upload document.pdf

# Upload to specific directory
fbcli upload project.zip /uploads

# Upload directory, ignoring .git folders
fbcli up -i "\\.git" ./my-project /projects
```

#### `download, down, dl [-i ignore] [-z] <remote_path> [local_path]`
Download files or directories from the remote server.

**Flags:**
- `-z`: Force zip compression for directories
- `-i <regex>`: Ignore files matching pattern

**Examples:**
```bash
# Download file
fbcli download /documents/report.pdf

# Download directory as zip
fbcli download -z /projects/webapp

# Download with ignore pattern
fbcli dl -i "node_modules" /source/project ./local-copy
```

### Directory Operations

#### `mkdir, md <remote_path>...`
Create one or more directories.

```bash
# Create single directory
fbcli mkdir /new-folder

# Create multiple directories
fbcli md /dir1 /dir2 /dir3
```

#### `rm, delete [-i ignore] <remote_path>...`
Delete files or directories.

```bash
# Delete single file
fbcli rm /old-file.txt

# Delete multiple items
fbcli delete /tmp/file1 /tmp/file2

# Delete with ignore pattern (delete all .log files except error.log)
fbcli rm -i "error\\.log" /logs/*.log
```

#### `rename, mv <old_path> <new_path>`
Rename or move files and directories.

```bash
# Rename file
fbcli rename /old-name.txt /new-name.txt

# Move file to different directory
fbcli mv /temp/file.txt /documents/file.txt
```

### Synchronization

#### `syncto, to [-i ignore] <local_path> <remote_path>`
Synchronize local directory to remote location.

```bash
# Basic sync
fbcli syncto ./local-folder /remote-backup

# Sync with ignore pattern
fbcli to -i "\\.(git|DS_Store)" ./project /projects/my-app
```

#### `syncfrom, from [-i ignore] <remote_path> <local_path>`
Synchronize remote directory to local location.

```bash
# Sync from remote
fbcli syncfrom /server-configs ./local-backup

# Sync ignoring temporary files
fbcli from -i "temp.*" /workspace ./local-workspace
```

### Configuration

#### `show`
Display current configuration and version information.

```bash
fbcli show
```

## 🔍 Advanced Features

### Regex Ignore Patterns

All commands support powerful regex-based ignore patterns with the `-i` flag:

```bash
# Ignore all log files
-i ".*\\.log$"

# Ignore temporary files and directories
-i "(temp|tmp|cache)"

# Ignore version control directories
-i "\\.(git|svn|hg)"

# Ignore multiple file types
-i "\\.(log|tmp|cache|bak)$"

# Ignore directories starting with dot
-i "^\\..*"
```

### Script-Friendly Output

Use the `-s` flag with `ls` for automation and scripting:

```bash
# Process each file in a loop
fbcli ls -s /data | while read filename; do
    echo "Processing $filename"
    fbcli download "/data/$filename" "./processed/"
done
```

### Zip Download Optimization

Directories are automatically downloaded as zip files for efficiency:

```bash
# Downloads as project.zip
fbcli download /large-project

# Force zip even for single files
fbcli download -z /single-file.txt
```

## 🧪 Testing

fbcli includes a comprehensive test suite with 100% cleanup:

```bash
# Run all tests
./testsuite/run-tests.bash

# Run specific test
./testsuite/test-upload.bash

# Run with verbose output
./testsuite/test-ls.bash
```

The test suite includes:
- Upload/download operations
- Directory synchronization
- Pattern matching and ignore functionality
- Command aliases verification
- Error handling scenarios
- Resource cleanup validation

## 🔧 Development

### Building

```bash
# Build for current platform
go build -o fbcli

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o fbcli-linux-amd64
GOOS=windows GOARCH=amd64 go build -o fbcli-windows-amd64.exe
GOOS=darwin GOARCH=amd64 go build -o fbcli-darwin-amd64
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run tests
go test ./...
```

## 📚 API Reference

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `FILEBROWSER_URL` | FileBrowser instance URL | Yes |
| `FILEBROWSER_USERNAME` | Login username | Yes* |
| `FILEBROWSER_PASSWORD` | Login password | Yes* |

*Will prompt interactively if not provided

### Exit Codes

| Code | Description |
|------|-------------|
| 0 | Success |
| 1 | General error (network, authentication, file operations) |

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Run tests (`./testsuite/run-tests.bash`)
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🚧 Roadmap

- [ ] Configuration file support
- [ ] Progress bars for large transfers
- [ ] Resume interrupted downloads
- [ ] Parallel upload/download
- [ ] Watch mode for continuous sync
- [ ] Plugin system for custom operations

## 🐛 Issues & Support

- Report bugs: [GitHub Issues](https://github.com/johnwmail/fbcli/issues)
- Feature requests: [GitHub Discussions](https://github.com/johnwmail/fbcli/discussions)
- Documentation: [Wiki](https://github.com/johnwmail/fbcli/wiki)

## 🙏 Acknowledgments

- [FileBrowser](https://filebrowser.org) for the excellent web-based file management interface
- The Go community for amazing tools and libraries
- Contributors who help improve this project

---

**Made with ❤️ for the FileBrowser community**
