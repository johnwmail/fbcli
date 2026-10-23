package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ergochat/readline"
	"golang.org/x/term"
)

var errShellExit = errors.New("shell exit")

// Shell is an sftp-style interactive session on top of the normal API client.
type Shell struct {
	client *Client

	remoteCWD         string
	previousRemoteCWD string

	localCWD         string
	previousLocalCWD string
}

// RunShell starts the interactive shell. When stdin is not a terminal the shell
// reads newline-delimited commands from stdin, which makes it usable in scripts:
//
//	printf 'pwd\nls /\nexit\n' | fbcli shell
func RunShell(client *Client) error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	shell := &Shell{
		client:    client,
		remoteCWD: "/",
		localCWD:  cwd,
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return shell.runInteractive()
	}
	return shell.runBatch(os.Stdin)
}

func (s *Shell) runInteractive() error {
	fmt.Printf("Connected to %s\n", s.client.Config.URL)
	fmt.Printf("Source: %s\n", s.client.Source)
	fmt.Println("Type 'help' for commands, 'exit' to quit.")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          s.prompt(),
		HistoryFile:     shellHistoryPath(),
		HistoryLimit:    1000,
		AutoComplete:    &shellCompleter{shell: s},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return err
	}
	defer func() {
		_ = rl.Close()
	}()

	for {
		line, err := rl.Readline()
		switch {
		case errors.Is(err, readline.ErrInterrupt):
			continue
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := s.execute(line); err != nil {
			if errors.Is(err, errShellExit) {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		rl.SetPrompt(s.prompt())
	}
}

func (s *Shell) runBatch(reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	failures := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := s.execute(line); err != nil {
			if errors.Is(err, errShellExit) {
				break
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			failures++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if failures > 0 {
		return fmt.Errorf("%d shell command(s) failed", failures)
	}
	return nil
}

func (s *Shell) execute(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if strings.HasPrefix(line, "#") {
		return nil
	}
	if strings.HasPrefix(line, "!") {
		return s.runLocalCommand(strings.TrimSpace(strings.TrimPrefix(line, "!")))
	}

	args, err := splitCommandLine(line)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "help", "?":
		printShellHelp(os.Stdout)
		return nil
	case "version":
		fmt.Printf("%s version %s\n", filepath.Base(os.Args[0]), version)
		return nil
	case "exit", "quit", "bye", "q":
		return errShellExit
	case "pwd":
		if len(args) != 1 {
			return fmt.Errorf("pwd takes no arguments")
		}
		fmt.Println(s.remoteCWD)
		return nil
	case "lpwd":
		if len(args) != 1 {
			return fmt.Errorf("lpwd takes no arguments")
		}
		fmt.Println(s.localCWD)
		return nil
	case "cd":
		if len(args) > 2 {
			return fmt.Errorf("cd takes one optional remote path")
		}
		return s.changeRemoteDir(optionArg(args, 1))
	case "lcd":
		if len(args) > 2 {
			return fmt.Errorf("lcd takes one optional local path")
		}
		return s.changeLocalDir(optionArg(args, 1))
	case "ls", "list", "dir":
		return s.shellLS(args[1:])
	case "lls":
		return s.shellLLS(args[1:])
	case "get":
		return s.shellGet(args[1:])
	case "put":
		return s.shellPut(args[1:])
	case "mkdir", "md":
		return s.shellMkdir(args[1:])
	case "rm", "delete":
		return s.shellRM(args[1:])
	case "rename", "mv":
		return s.shellRename(args[1:])
	case "syncto", "to":
		return s.shellSyncTo(args[1:])
	case "syncfrom", "from":
		return s.shellSyncFrom(args[1:])
	default:
		return fmt.Errorf("unknown command %q (type 'help' for commands)", args[0])
	}
}

func optionArg(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}

func (s *Shell) prompt() string {
	source := s.client.Source
	if source == "" {
		source = "auto"
	}
	return fmt.Sprintf("fbcli[%s]:%s> ", source, s.remoteCWD)
}

type shellOptions struct {
	long   bool
	script bool
	zip    bool
	ignore string
}

// parseShellOptions parses sftp-like flags. allowed contains the supported option
// letters: l (long), s (script), i (ignore), r (recursive), z (zip), p (parents).
func parseShellOptions(args []string, allowed string) (shellOptions, []string, error) {
	var options shellOptions
	var rest []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			rest = append(rest, args[i+1:]...)
			return options, rest, nil
		case arg == "-l" || arg == "--long":
			if !strings.Contains(allowed, "l") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
			options.long = true
		case arg == "-s" || arg == "--script":
			if !strings.Contains(allowed, "s") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
			options.script = true
		case arg == "-r" || arg == "--recursive":
			if !strings.Contains(allowed, "r") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
		case arg == "-z" || arg == "--zip":
			if !strings.Contains(allowed, "z") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
			options.zip = true
		case arg == "-p" || arg == "--parents":
			if !strings.Contains(allowed, "p") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
		case arg == "-i" || arg == "--ignore":
			if !strings.Contains(allowed, "i") {
				return options, nil, fmt.Errorf("option %s is not valid for this command", arg)
			}
			if i+1 >= len(args) {
				return options, nil, fmt.Errorf("%s requires a regular expression", arg)
			}
			i++
			options.ignore = args[i]
		case strings.HasPrefix(arg, "--ignore="):
			if !strings.Contains(allowed, "i") {
				return options, nil, fmt.Errorf("option --ignore is not valid for this command")
			}
			options.ignore = strings.TrimPrefix(arg, "--ignore=")
		case strings.HasPrefix(arg, "-") && arg != "-":
			return options, nil, fmt.Errorf("unknown option %q", arg)
		default:
			rest = append(rest, arg)
		}
	}
	return options, rest, nil
}

func (s *Shell) changeRemoteDir(arg string) error {
	if arg == "-" {
		if s.previousRemoteCWD == "" {
			return fmt.Errorf("no previous remote directory")
		}
		s.remoteCWD, s.previousRemoteCWD = s.previousRemoteCWD, s.remoteCWD
		fmt.Println(s.remoteCWD)
		return nil
	}
	if arg == "" || arg == "~" {
		arg = "/"
	}
	target := s.remotePath(arg)
	item, err := s.client.getResource(target)
	if err != nil {
		return err
	}
	if !item.IsDir {
		return fmt.Errorf("%s is not a directory", target)
	}
	s.previousRemoteCWD = s.remoteCWD
	s.remoteCWD = target
	return nil
}

func (s *Shell) changeLocalDir(arg string) error {
	if arg == "-" {
		if s.previousLocalCWD == "" {
			return fmt.Errorf("no previous local directory")
		}
		s.localCWD, s.previousLocalCWD = s.previousLocalCWD, s.localCWD
		fmt.Println(s.localCWD)
		return nil
	}

	target := s.localPath(arg)
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", target)
	}
	s.previousLocalCWD = s.localCWD
	s.localCWD = target
	return nil
}

func (s *Shell) shellLS(args []string) error {
	options, rest, err := parseShellOptions(args, "lsi")
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return fmt.Errorf("ls takes one optional remote path")
	}
	target := s.remoteCWD
	if len(rest) == 1 {
		target = s.remotePath(rest[0])
	}
	return s.client.List(target, options.ignore, options.long, options.script)
}

func (s *Shell) shellLLS(args []string) error {
	options, rest, err := parseShellOptions(args, "l")
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return fmt.Errorf("lls takes one optional local path")
	}
	target := s.localCWD
	if len(rest) == 1 {
		target = s.localPath(rest[0])
	}
	return s.listLocal(target, options.long)
}

func (s *Shell) shellGet(args []string) error {
	options, rest, err := parseShellOptions(args, "irz")
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: get [-r] [-z] [-i regex] <remote_path> [local_path]")
	}
	if options.zip && options.ignore != "" {
		return fmt.Errorf("-z (zip) and -i (ignore) cannot be used together")
	}

	remote := s.remotePath(rest[0])
	local := s.localCWD
	if len(rest) == 2 {
		local = s.localPath(rest[1])
	}

	if options.zip {
		return s.client.Download(remote, local, "", true)
	}
	return s.client.downloadRecursive(remote, local, options.ignore)
}

func (s *Shell) shellPut(args []string) error {
	options, rest, err := parseShellOptions(args, "ir")
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: put [-r] [-i regex] <local_path> [remote_path]")
	}

	local := s.localPath(rest[0])
	info, err := os.Stat(local)
	if err != nil {
		return fmt.Errorf("access local path %q: %w", local, err)
	}

	remote := s.remoteCWD
	if len(rest) == 2 {
		remote = s.remotePath(rest[1])
	}

	if info.IsDir() {
		return s.client.Upload(local, remote, options.ignore)
	}
	if options.ignore != "" {
		ignore, err := compileIgnore(options.ignore)
		if err != nil {
			return err
		}
		if ignore.MatchString(info.Name()) {
			fmt.Printf("Ignoring file: %s\n", info.Name())
			return nil
		}
	}

	// sftp semantics: an explicit destination that is not an existing directory is
	// the target file name, including when it does not exist yet.
	if len(rest) == 2 {
		item, lookupErr := s.client.getResource(remote)
		if lookupErr == nil && item.IsDir {
			return s.client.Upload(local, remote, "")
		}
		if err := s.client.uploadFileToPath(local, remote); err != nil {
			return err
		}
		fmt.Println("Upload complete.")
		return nil
	}
	if err := s.client.uploadFileToPath(local, path.Join(remote, info.Name())); err != nil {
		return err
	}
	fmt.Println("Upload complete.")
	return nil
}

func (s *Shell) shellMkdir(args []string) error {
	_, rest, err := parseShellOptions(args, "p")
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf("mkdir requires at least one remote path")
	}
	for _, arg := range rest {
		if err := s.client.Mkdir(s.remotePath(arg)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Shell) shellRM(args []string) error {
	options, rest, err := parseShellOptions(args, "i")
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf("rm requires at least one remote path")
	}
	for _, arg := range rest {
		if err := s.client.DeleteIgnore(s.remotePath(arg), options.ignore); err != nil {
			return err
		}
	}
	return nil
}

func (s *Shell) shellRename(args []string) error {
	_, rest, err := parseShellOptions(args, "")
	if err != nil {
		return err
	}
	if len(rest) != 2 {
		return fmt.Errorf("usage: rename <old_remote_path> <new_remote_path>")
	}
	return s.client.Rename(s.remotePath(rest[0]), s.remotePath(rest[1]))
}

func (s *Shell) shellSyncTo(args []string) error {
	options, rest, err := parseShellOptions(args, "i")
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: syncto [-i regex] <local_path> [remote_path]")
	}
	remote := s.remoteCWD
	if len(rest) == 2 {
		remote = s.remotePath(rest[1])
	}
	return s.client.SyncTo(s.localPath(rest[0]), remote, options.ignore)
}

func (s *Shell) shellSyncFrom(args []string) error {
	options, rest, err := parseShellOptions(args, "i")
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: syncfrom [-i regex] <remote_path> [local_path]")
	}
	local := s.localCWD
	if len(rest) == 2 {
		local = s.localPath(rest[1])
	}
	return s.client.SyncFrom(s.remotePath(rest[0]), local, options.ignore)
}

func (s *Shell) listLocal(target string, long bool) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	var items []Resource
	if !info.IsDir() {
		items = []Resource{{
			Name:     info.Name(),
			Size:     info.Size(),
			Modified: info.ModTime(),
		}}
	} else {
		entries, err := os.ReadDir(target)
		if err != nil {
			return err
		}
		items = make([]Resource, 0, len(entries))
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}
			items = append(items, Resource{
				Name:     entry.Name(),
				Size:     entryInfo.Size(),
				Modified: entryInfo.ModTime(),
				IsDir:    entry.IsDir(),
			})
		}
	}

	sortResources(items)
	if long {
		printLongList(items)
	} else {
		printShortList(items, false)
	}
	return nil
}

func (s *Shell) runLocalCommand(command string) error {
	var cmd *exec.Cmd
	if command == "" {
		shell := os.Getenv("SHELL")
		if shell == "" {
			if runtime.GOOS == "windows" {
				shell = "cmd"
			} else {
				shell = "sh"
			}
		}
		cmd = exec.Command(shell)
	} else if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Dir = s.localCWD
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Shell) remotePath(arg string) string {
	if arg == "" {
		return normalizeRemotePath(s.remoteCWD)
	}
	if strings.HasPrefix(arg, "/") {
		return normalizeRemotePath(arg)
	}
	return normalizeRemotePath(path.Join(s.remoteCWD, arg))
}

func (s *Shell) localPath(arg string) string {
	if arg == "" {
		return s.localCWD
	}
	if arg == "~" || strings.HasPrefix(arg, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			if arg == "~" {
				return home
			}
			return filepath.Join(home, arg[2:])
		}
	}
	if filepath.IsAbs(arg) {
		return filepath.Clean(arg)
	}
	return filepath.Join(s.localCWD, arg)
}

func splitCommandLine(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	escaped := false
	started := false

	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			started = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t':
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	if escaped {
		current.WriteRune('\\')
	}
	if started {
		args = append(args, current.String())
	}
	return args, nil
}

func shellHistoryPath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		return ""
	}
	path := filepath.Join(dir, "fbcli", "history")
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	return path
}

func printShellHelp(out io.Writer) {
	_, _ = fmt.Fprint(out, `fbcli interactive shell (FileBrowser Quantum v2)

Remote commands:
  pwd                         Print the current remote directory
  cd [path]                   Change remote directory ('cd' goes to the source root, '-' returns)
  ls [-l] [-s] [-i regex] [path]
                              List a remote directory
  mkdir [-p] <path>...        Create remote directories
  rm [-i regex] <path>...     Delete remote files/directories
  rename <old> <new>          Rename or move a remote path
  get [-r] [-z] [-i regex] <remote> [local]
                              Download a file or directory
  put [-r] [-i regex] <local> [remote]
                              Upload a file or directory
  syncto [-i regex] <local> [remote]
                              Mirror a local directory to the remote side
  syncfrom [-i regex] <remote> [local]
                              Mirror a remote directory to the local side

Local commands:
  lpwd                        Print the current local directory
  lcd [path]                  Change local directory ('-' returns)
  lls [-l] [path]             List a local directory
  !command                    Run a local shell command
  !                           Start a local interactive shell

Other:
  help, ?                     Show this help
  version                     Show the fbcli version
  exit, quit, bye             Leave the shell

Paths are relative to the current remote or local directory unless they start
with '/'. Local paths may start with '~'. Use quotes or backslash escapes for
names containing spaces.
`)
}

type shellCompleter struct {
	shell *Shell
}

type completionKind int

const (
	completeNone completionKind = iota
	completeRemote
	completeLocal
)

func (c *shellCompleter) Do(line []rune, pos int) ([][]rune, int) {
	if pos > len(line) {
		pos = len(line)
	}
	prefix := string(line[:pos])
	if strings.ContainsAny(prefix, `"'`) {
		return nil, 0
	}

	tokenStart := strings.LastIndexAny(prefix, " \t") + 1
	token := prefix[tokenStart:]
	tokenLen := utf8.RuneCountInString(token)
	previous := strings.Fields(prefix[:tokenStart])

	if len(previous) == 0 {
		return completionSuffixes(shellCommandNames(), token, tokenLen), tokenLen
	}

	command := strings.ToLower(previous[0])
	if command == "!" {
		return nil, 0
	}

	if strings.HasPrefix(token, "-") {
		return completionSuffixes(shellFlagNames(command), token, tokenLen), tokenLen
	}

	argIndex := shellArgumentIndex(previous[1:])
	switch completionKindFor(command, argIndex) {
	case completeRemote:
		return completionSuffixes(c.shell.completeRemote(token), token, tokenLen), tokenLen
	case completeLocal:
		return completionSuffixes(c.shell.completeLocal(token), token, tokenLen), tokenLen
	default:
		return nil, 0
	}
}

func completionSuffixes(candidates []string, token string, tokenLen int) [][]rune {
	result := make([][]rune, 0, len(candidates))
	for _, candidate := range candidates {
		if !strings.HasPrefix(candidate, token) || candidate == token {
			continue
		}
		result = append(result, []rune(candidate[len(token):]))
	}
	return result
}

func shellCommandNames() []string {
	return []string{
		"bye", "cd", "delete", "dir", "exit", "get", "help", "lcd", "list",
		"lls", "lpwd", "ls", "md", "mkdir", "mv", "pwd", "put", "q", "quit",
		"rename", "rm", "syncto", "syncfrom", "to", "from", "version",
	}
}

func shellFlagNames(command string) []string {
	switch command {
	case "ls", "list", "dir":
		return []string{"-l", "-s", "-i"}
	case "lls":
		return []string{"-l"}
	case "get":
		return []string{"-r", "-z", "-i"}
	case "put":
		return []string{"-r", "-i"}
	case "mkdir", "md":
		return []string{"-p"}
	case "rm", "delete":
		return []string{"-i"}
	case "syncto", "to", "syncfrom", "from":
		return []string{"-i"}
	}
	return nil
}

func shellArgumentIndex(previous []string) int {
	count := 0
	for i := 0; i < len(previous); i++ {
		arg := previous[i]
		if strings.HasPrefix(arg, "-") {
			if (arg == "-i" || arg == "--ignore") && i+1 < len(previous) {
				i++
			}
			continue
		}
		count++
	}
	return count
}

func completionKindFor(command string, argIndex int) completionKind {
	switch command {
	case "cd", "ls", "list", "dir", "mkdir", "md", "rm", "delete", "rename", "mv":
		return completeRemote
	case "lcd", "lls":
		return completeLocal
	case "get", "syncfrom", "from":
		if argIndex == 0 {
			return completeRemote
		}
		if argIndex == 1 {
			return completeLocal
		}
	case "put", "syncto", "to":
		if argIndex == 0 {
			return completeLocal
		}
		if argIndex == 1 {
			return completeRemote
		}
	}
	return completeNone
}

func (s *Shell) completeRemote(token string) []string {
	typedDir := ""
	if index := strings.LastIndex(token, "/"); index >= 0 {
		typedDir = token[:index+1]
	}
	base := token[len(typedDir):]

	dir := s.remoteCWD
	if typedDir != "" {
		if strings.HasPrefix(typedDir, "/") {
			dir = normalizeRemotePath(typedDir)
		} else {
			dir = normalizeRemotePath(path.Join(s.remoteCWD, typedDir))
		}
	}

	items, err := s.client.listRemote(dir)
	if err != nil {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	candidates := make([]string, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.Name, base) {
			continue
		}
		name := escapeCompletionName(item.Name)
		candidate := typedDir + name
		if item.IsDir {
			candidate += "/"
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (s *Shell) completeLocal(token string) []string {
	if strings.HasPrefix(token, "~") {
		return nil
	}

	typedDir := ""
	if index := strings.LastIndexAny(token, `/\`); index >= 0 {
		typedDir = token[:index+1]
	}
	base := token[len(typedDir):]

	dirArg := typedDir
	if dirArg == "" {
		dirArg = "."
	}
	entries, err := os.ReadDir(s.localPath(dirArg))
	if err != nil {
		return nil
	}

	candidates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		name := escapeCompletionName(entry.Name())
		candidate := typedDir + name
		if entry.IsDir() {
			candidate += string(os.PathSeparator)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func escapeCompletionName(name string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		` `, `\ `,
		"\t", `\t`,
		`"`, `\"`,
		`'`, `\'`,
	)
	return replacer.Replace(name)
}
