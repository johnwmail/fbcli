package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

var version = "dev" // set by the build process
var userAgent = "fbcli/" + version

var errHelpRequested = errors.New("help requested")

type commandOptions struct {
	ignore string
	source string
	zip    bool
	script bool
	long   bool
}

func main() {
	progName := filepath.Base(os.Args[0])
	if len(os.Args) < 2 {
		printUsage(os.Stderr, progName)
		os.Exit(1)
	}

	command := os.Args[1]
	options, args, err := parseOptions(os.Args[2:])
	if err != nil {
		if errors.Is(err, errHelpRequested) {
			printUsage(os.Stdout, progName)
			return
		}
		exitWithError("%v", err)
	}

	switch command {
	case "help", "-h", "--help":
		printUsage(os.Stdout, progName)
		return
	case "version", "-v", "--version":
		fmt.Printf("%s version %s\n", progName, version)
		return
	}

	config := configFromEnv()
	if options.source != "" {
		config.Source = options.source
	}
	client := NewClient(config)

	if command == "show" {
		if err := client.loadConfig(false); err != nil {
			exitWithError("%v", err)
		}
		client.showConfig()
		return
	}

	if err := client.loadConfig(true); err != nil {
		exitWithError("%v", err)
	}

	usedCache := client.LoadSession()
	if !usedCache {
		if err := client.Login(); err != nil {
			exitWithError("Login failed: %v", err)
		}
		client.SaveSession()
	}

	if err := client.ResolveSource(); err != nil {
		if !usedCache || !isUnauthorized(err) {
			exitWithError("%v", err)
		}
		// A cached session may have expired or been revoked; log in once more.
		client.Token = ""
		if err := client.Login(); err != nil {
			exitWithError("Login failed: %v", err)
		}
		client.SaveSession()
		if err := client.ResolveSource(); err != nil {
			exitWithError("%v", err)
		}
	}

	if err := runCommand(client, command, options, args); err != nil {
		exitWithError("%v", err)
	}
}

func runCommand(client *Client, command string, options commandOptions, args []string) error {
	switch command {
	case "ls", "list", "dir":
		remotePath := "/"
		if len(args) > 0 {
			remotePath = strings.Join(args, " ")
		}
		long := options.long || command == "list" || command == "dir"
		return client.List(remotePath, options.ignore, long, options.script)

	case "upload", "up":
		if len(args) < 1 || len(args) > 2 {
			return usageError(command)
		}
		remoteDir := "/"
		if len(args) == 2 {
			remoteDir = args[1]
		}
		return client.Upload(args[0], remoteDir, options.ignore)

	case "download", "down", "dl":
		if len(args) < 1 || len(args) > 2 {
			return usageError(command)
		}
		localPath := ""
		if len(args) == 2 {
			localPath = args[1]
		}
		return client.Download(args[0], localPath, options.ignore, options.zip)

	case "mkdir", "md":
		if len(args) < 1 {
			return usageError(command)
		}
		for _, remotePath := range args {
			if err := client.Mkdir(remotePath); err != nil {
				return err
			}
		}
		return nil

	case "rm", "delete":
		if len(args) < 1 {
			return usageError(command)
		}
		for _, remotePath := range args {
			if err := client.DeleteIgnore(remotePath, options.ignore); err != nil {
				return err
			}
		}
		return nil

	case "rename", "mv":
		if len(args) != 2 {
			return usageError(command)
		}
		return client.Rename(args[0], args[1])

	case "syncto", "to":
		if len(args) != 2 {
			return usageError(command)
		}
		return client.SyncTo(args[0], args[1], options.ignore)

	case "syncfrom", "from":
		if len(args) != 2 {
			return usageError(command)
		}
		return client.SyncFrom(args[0], args[1], options.ignore)

	default:
		return fmt.Errorf("unknown command %q (run with --help for usage)", command)
	}
}

func configFromEnv() Config {
	return Config{
		URL:      os.Getenv("FILEBROWSER_URL"),
		Username: os.Getenv("FILEBROWSER_USERNAME"),
		Password: os.Getenv("FILEBROWSER_PASSWORD"),
		TOTP:     os.Getenv("FILEBROWSER_TOTP"),
		Source:   os.Getenv("FILEBROWSER_SOURCE"),
	}
}

// loadConfig fills in missing values interactively. Passwords are only requested when
// they are actually needed (not for "show").
func (c *Client) loadConfig(requirePassword bool) error {
	reader := bufio.NewReader(os.Stdin)

	if c.Config.URL == "" {
		fmt.Print("Enter File Browser URL: ")
		value, err := readLine(reader)
		if err != nil {
			return err
		}
		c.Config.URL = value
	}
	if c.Config.URL == "" {
		return fmt.Errorf("a server URL is required (set FILEBROWSER_URL)")
	}

	if c.Config.Username == "" {
		fmt.Print("Enter Username: ")
		value, err := readLine(reader)
		if err != nil {
			return err
		}
		c.Config.Username = value
	}
	if c.Config.Username == "" {
		return fmt.Errorf("a username is required (set FILEBROWSER_USERNAME)")
	}

	if !requirePassword {
		return nil
	}
	if c.Config.Password == "" {
		fmt.Print("Enter Password: ")
		value, err := readPassword(reader)
		if err != nil {
			return err
		}
		c.Config.Password = value
	}
	return nil
}

func readLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func readPassword(reader *bufio.Reader) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		value, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}

	value, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (c *Client) showConfig() {
	source := c.Config.Source
	if source == "" {
		source = "(auto)"
	}
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("URL: %s\n", c.Config.URL)
	fmt.Printf("Username: %s\n", c.Config.Username)
	fmt.Printf("Source: %s\n", source)
	fmt.Printf("Password: %s\n", redactPassword(c.Config.Password))
}

func parseOptions(args []string) (commandOptions, []string, error) {
	options := commandOptions{}
	positional := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			return options, positional, nil
		case arg == "-i" || arg == "--ignore":
			if i+1 >= len(args) {
				return options, nil, fmt.Errorf("%s requires a regular expression", arg)
			}
			i++
			options.ignore = args[i]
		case strings.HasPrefix(arg, "--ignore="):
			options.ignore = strings.TrimPrefix(arg, "--ignore=")
		case arg == "-S" || arg == "--source":
			if i+1 >= len(args) {
				return options, nil, fmt.Errorf("%s requires a source name", arg)
			}
			i++
			options.source = args[i]
		case strings.HasPrefix(arg, "--source="):
			options.source = strings.TrimPrefix(arg, "--source=")
		case arg == "-z" || arg == "--zip":
			options.zip = true
		case arg == "-s" || arg == "--script":
			options.script = true
		case arg == "-l" || arg == "--long":
			options.long = true
		case arg == "-h" || arg == "--help":
			return options, nil, errHelpRequested
		case strings.HasPrefix(arg, "-") && arg != "-":
			return options, nil, fmt.Errorf("unknown option %q", arg)
		default:
			positional = append(positional, arg)
		}
	}

	return options, positional, nil
}

func usageError(command string) error {
	return fmt.Errorf("invalid number of arguments for %q (run with --help for usage)", command)
}

func printUsage(out io.Writer, progName string) {
	_, _ = fmt.Fprintf(out, "%s version %s\n", progName, version)
	_, _ = fmt.Fprintf(out, "Usage: %s <command> [options] [arguments...]\n", progName)
	_, _ = fmt.Fprint(out, `
Commands:
  ls [-i ignore] [-l] [-s] [remote_path]        List files/directories
                                                -l: detailed view with sizes and dates
                                                -s: script-friendly output (one per line)
  list, dir [-i ignore] [remote_path]           Detailed listing (like ls -l)
  upload, up [-i ignore] <local_path> [remote_dir]
                                                Upload a file or directory
  download, down, dl [-i ignore] [-z] <remote_path> [local_path]
                                                Download a file or directory as a zip
  mkdir, md <remote_path>...                    Create one or more directories
  rm, delete [-i ignore] <remote_path>...       Delete one or more files or directories
  rename, mv <old_path> <new_path>              Rename or move a file or directory
  syncto, to [-i ignore] <local_path> <remote_path>
                                                Make the remote path mirror the local path
  syncfrom, from [-i ignore] <remote_path> <local_path>
                                                Make the local path mirror the remote path
  show                                          Show the current configuration
  help, version                                 Show this help / version

Options:
  -i, --ignore <regex>    Skip entries whose name matches the regular expression
  -S, --source <name>     Override FILEBROWSER_SOURCE for this command
  -z, --zip               Download a directory as a zip archive
  -s, --script            Print plain names suitable for shell pipelines
  -l, --long              Detailed listing format
  --                      Treat all following arguments as paths

Environment:
  FILEBROWSER_URL         Server URL (required)
  FILEBROWSER_USERNAME    Login username (prompted if omitted)
  FILEBROWSER_PASSWORD    Login password (prompted if omitted)
  FILEBROWSER_SOURCE      Source name (auto-detected when only one is available)
  FILEBROWSER_TOTP        TOTP code when two-factor authentication is enabled

FileBrowser Quantum v2.x is required. The source defaults to the first source the
account can access; use FILEBROWSER_SOURCE when multiple sources are configured.
`)
}

func exitWithError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(os.Stderr)
	}
	os.Exit(1)
}
