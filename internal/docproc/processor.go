package docproc

import (
	"regexp"
	"strings"
)

// Heading represents a markdown heading.
type Heading struct {
	Level int
	Text  string
	Line  int
}

// NavigationFile is a file entry extracted from README navigation.
type NavigationFile struct {
	Name        string
	Description string
	Link        string
}

// Navigation holds the parsed README navigation structure.
type Navigation struct {
	Overview string
	Files    []NavigationFile
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var tableSepRe = regexp.MustCompile(`^\|[\s:]*-+[\s:|-]*\|$`)

// ExtractNavigationFromReadme parses a README.md for navigation structure.
func ExtractNavigationFromReadme(content string) Navigation {
	nav := Navigation{}
	lines := strings.Split(content, "\n")

	inNav := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)

		if strings.Contains(line, "##") &&
			(strings.Contains(strings.ToLower(line), "navigation") ||
				strings.Contains(strings.ToLower(line), "contents")) {
			inNav = true
			continue
		}

		if inNav {
			if strings.HasPrefix(line, "##") && !strings.Contains(strings.ToLower(line), "navigation") {
				break
			}
			// Table row
			if strings.Contains(line, "|") && !tableSepRe.MatchString(line) {
				parts := splitPipe(line)
				if len(parts) >= 2 {
					entry := NavigationFile{Name: parts[0], Description: parts[1]}
					if len(parts) >= 3 {
						entry.Link = parts[2]
					}
					nav.Files = append(nav.Files, entry)
				}
			} else if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				item := line[2:]
				if m := mdLinkRe.FindStringSubmatch(item); m != nil {
					nav.Files = append(nav.Files, NavigationFile{Name: m[1], Link: m[2]})
				} else {
					nav.Files = append(nav.Files, NavigationFile{Name: item})
				}
			}
		}
	}

	// Extract overview from first non-heading paragraph
	var overviewParts []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			if len(overviewParts) > 0 {
				break
			}
			continue
		}
		overviewParts = append(overviewParts, line)
		if len(overviewParts) >= 3 {
			break
		}
	}
	nav.Overview = strings.Join(overviewParts, " ")
	return nav
}

// ExtractTableOfContents returns all headings from markdown content.
func ExtractTableOfContents(content string) []Heading {
	var toc []Heading
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "#") {
			continue
		}
		level := 0
		for _, c := range stripped {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		if level > 6 {
			continue
		}
		text := strings.TrimSpace(stripped[level:])
		if text != "" {
			toc = append(toc, Heading{Level: level, Text: text, Line: i})
		}
	}
	return toc
}

// ExtractSummary returns the first paragraph after the H1 heading.
func ExtractSummary(content string, maxLen int) string {
	lines := strings.Split(content, "\n")
	startIdx := 0

	// Skip frontmatter
	if strings.HasPrefix(content, "---") {
		end := strings.Index(content[3:], "---")
		if end >= 0 {
			prefix := content[:end+6]
			startIdx = strings.Count(prefix, "\n")
		}
	}

	var parts []string
	for i := startIdx; i < len(lines) && i < startIdx+20; i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "# ") {
			continue
		}
		if line == "" || strings.HasPrefix(line, "---") {
			if len(parts) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "##") {
			break
		}
		parts = append(parts, line)
		if len(strings.Join(parts, " ")) > maxLen {
			break
		}
	}

	summary := strings.Join(parts, " ")
	if len(summary) > maxLen {
		cut := summary[:maxLen]
		if idx := strings.LastIndex(cut, " "); idx > maxLen*3/4 {
			cut = cut[:idx]
		}
		summary = cut + "..."
	}
	return summary
}

// ExtractSection returns the content of a section identified by heading text.
// Matches case-insensitively and returns content until the next heading of equal or higher level.
// Returns empty string if heading not found.
func ExtractSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	needle := strings.ToLower(strings.TrimSpace(heading))

	targetIdx := -1
	targetLevel := 0
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if !strings.HasPrefix(stripped, "#") {
			continue
		}
		level := 0
		for _, c := range stripped {
			if c == '#' {
				level++
			} else {
				break
			}
		}
		text := strings.ToLower(strings.TrimSpace(stripped[level:]))
		if text == needle || strings.Contains(text, needle) {
			targetIdx = i
			targetLevel = level
			break
		}
	}

	if targetIdx == -1 {
		return ""
	}

	var result []string
	result = append(result, lines[targetIdx])
	for i := targetIdx + 1; i < len(lines); i++ {
		stripped := strings.TrimSpace(lines[i])
		if strings.HasPrefix(stripped, "#") {
			level := 0
			for _, c := range stripped {
				if c == '#' {
					level++
				} else {
					break
				}
			}
			if level <= targetLevel {
				break
			}
		}
		result = append(result, lines[i])
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

func splitPipe(line string) []string {
	var parts []string
	for _, p := range strings.Split(line, "|") {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
