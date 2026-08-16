package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var releaseVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

func main() {
	changelogPath := flag.String("changelog", "CHANGELOG.md", "path to the changelog")
	version := flag.String("version", "", "release version without a leading v")
	outputPath := flag.String("output", "release-notes.md", "path for extracted notes")
	flag.Parse()

	if !releaseVersionPattern.MatchString(*version) {
		fail("version must use dotted numeric form without a leading v")
	}

	content, errRead := os.ReadFile(*changelogPath)
	if errRead != nil {
		fail("read changelog: %v", errRead)
	}

	notes, errExtract := extractReleaseNotes(string(content), *version)
	if errExtract != nil {
		fail("extract release notes: %v", errExtract)
	}
	if errWrite := os.WriteFile(*outputPath, []byte(notes+"\n"), 0o644); errWrite != nil {
		fail("write release notes: %v", errWrite)
	}
}

func extractReleaseNotes(changelog, version string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n")
	headerPrefix := "## [" + version + "]"
	start := -1

	for index, line := range lines {
		if strings.HasPrefix(line, headerPrefix) {
			start = index + 1
			break
		}
	}
	if start == -1 {
		return "", fmt.Errorf("missing %s section", headerPrefix)
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "## [") {
			end = index
			break
		}
	}

	notes := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if notes == "" {
		return "", fmt.Errorf("%s section is empty", headerPrefix)
	}
	return notes, nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
