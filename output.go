package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

const (
	minColumnWidth = 16
	maxNameWidth   = 60
	minNameWidth   = 30
)

func sortResources(items []Resource) {
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.Modified.Equal(right.Modified) {
			return left.Name < right.Name
		}
		return left.Modified.After(right.Modified)
	})
}

func printShortList(items []Resource, script bool) {
	if script {
		for _, item := range items {
			fmt.Println(displayName(item, false))
		}
		return
	}

	if len(items) == 0 {
		return
	}

	useColor := term.IsTerminal(int(os.Stdout.Fd()))
	names := make([]string, len(items))
	maxLen := 0
	for i, item := range items {
		name := displayName(item, useColor)
		names[i] = name
		if visible := utf8.RuneCountInString(stripANSI(name)); visible > maxLen {
			maxLen = visible
		}
	}
	if maxLen < minColumnWidth {
		maxLen = minColumnWidth
	}

	width := terminalWidth()
	columnWidth := maxLen + 2
	columns := width / columnWidth
	if columns < 1 {
		columns = 1
	}
	rows := (len(names) + columns - 1) / columns

	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			index := column*rows + row
			if index >= len(names) {
				continue
			}
			name := names[index]
			visible := utf8.RuneCountInString(stripANSI(name))
			padding := columnWidth - visible
			if padding < 1 {
				padding = 1
			}
			fmt.Print(name, strings.Repeat(" ", padding))
		}
		fmt.Println()
	}
}

func printLongList(items []Resource) {
	maxName := len("Name")
	for _, item := range items {
		visible := utf8.RuneCountInString(displayName(item, false))
		if visible > maxName {
			maxName = visible
		}
	}
	if maxName > maxNameWidth {
		maxName = maxNameWidth
	}
	if maxName < minNameWidth {
		maxName = minNameWidth
	}

	fmt.Printf("%-*s %-19s %-12s\n", maxName, "Name", "Modified", "Size")
	fmt.Printf("%-*s %-19s %-12s\n", maxName,
		strings.Repeat("-", maxName), strings.Repeat("-", 19), strings.Repeat("-", 12))

	useColor := term.IsTerminal(int(os.Stdout.Fd()))
	for _, item := range items {
		name := displayName(item, useColor)
		visible := utf8.RuneCountInString(stripANSI(name))
		padding := maxName - visible
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("%s%s %-19s %-12d\n", name, strings.Repeat(" ", padding),
			formatModified(item), item.Size)
	}
}

func displayName(item Resource, useColor bool) string {
	name := strings.ReplaceAll(item.Name, "\n", "")
	name = strings.ReplaceAll(name, "\r", "")
	if item.IsDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	if useColor && item.IsDir {
		return colorBlue + name + colorReset
	}
	return name
}

func formatModified(item Resource) string {
	if item.Modified.IsZero() {
		return ""
	}
	return item.Modified.Local().Format("2006-01-02 15:04:05")
}

// stripANSI removes ANSI color escapes for width calculations.
func stripANSI(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	inEscape := false
	for _, r := range value {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func terminalWidth() int {
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		return width
	}
	if columns := os.Getenv("COLUMNS"); columns != "" {
		var width int
		if _, err := fmt.Sscanf(columns, "%d", &width); err == nil && width > 0 {
			return width
		}
	}
	return 80
}

func hashReader(reader io.Reader) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func redactPassword(password string) string {
	length := len(password)
	switch length {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return "**"
	default:
		return string(password[0]) + strings.Repeat("*", length-2) + string(password[length-1])
	}
}
