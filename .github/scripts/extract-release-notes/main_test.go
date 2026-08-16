package main

import (
	"strings"
	"testing"
)

func TestExtractReleaseNotes(t *testing.T) {
	changelog := "# Changelog\r\n\r\n## [Unreleased]\r\n\r\n## [1.2.3] - 2026-08-16\r\n\r\n### Added\r\n\r\n- Safe release notes.\r\n\r\n## [1.2.2] - 2026-08-15\r\n\r\n- Older notes.\r\n"

	notes, errExtract := extractReleaseNotes(changelog, "1.2.3")
	if errExtract != nil {
		t.Fatalf("extract release notes: %v", errExtract)
	}
	if strings.Contains(notes, "Older notes") {
		t.Fatalf("notes crossed into the previous release: %q", notes)
	}
	if !strings.Contains(notes, "Safe release notes") {
		t.Fatalf("notes omitted current release content: %q", notes)
	}
}

func TestExtractReleaseNotesRejectsMissingOrEmptySection(t *testing.T) {
	for name, testCase := range map[string]struct {
		changelog string
		version   string
	}{
		"missing": {
			changelog: "# Changelog\n\n## [1.0.0]\n\n- Notes.\n",
			version:   "1.2.4",
		},
		"empty": {
			changelog: "# Changelog\n\n## [1.2.3]\n\n## [1.2.2]\n\n- Notes.\n",
			version:   "1.2.3",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, errExtract := extractReleaseNotes(testCase.changelog, testCase.version); errExtract == nil {
				t.Fatal("expected extraction to fail")
			}
		})
	}
}
