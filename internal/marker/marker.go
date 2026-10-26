package marker

import (
	"fmt"
	"strings"
)

// StartTag returns the canonical HTML opening marker for markerID.
func StartTag(markerID string) string {
	return fmt.Sprintf("<!-- %s -->", strings.TrimSpace(markerID))
}

// EndTag returns the canonical HTML closing marker for markerID.
func EndTag(markerID string) string {
	return fmt.Sprintf("<!-- /%s -->", strings.TrimSpace(markerID))
}

// EnsureTags wraps blockContent in StartTag(markerID) and EndTag(markerID)
// if those tags are not already present at the outer boundary.
func EnsureTags(markerID, blockContent string) string {
	start := StartTag(markerID)
	end := EndTag(markerID)
	trimmed := strings.TrimSpace(blockContent)
	if strings.HasPrefix(trimmed, start) && strings.HasSuffix(trimmed, end) {
		return trimmed
	}
	if trimmed == "" {
		return start + "\n" + end
	}
	return start + "\n" + trimmed + "\n" + end
}

// HasBlock reports whether content contains a complete, properly ordered
// <!-- markerID --> ... <!-- /markerID --> section.
func HasBlock(content, markerID string) bool {
	start := StartTag(markerID)
	end := EndTag(markerID)
	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return false
	}
	endIdx := strings.Index(content[startIdx+len(start):], end)
	return endIdx >= 0
}

// ExtractBlock returns the full marked block (including start and end tags)
// and true when present, or ("", false) when absent.
func ExtractBlock(content, markerID string) (string, bool) {
	start := StartTag(markerID)
	end := EndTag(markerID)
	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return "", false
	}
	relEnd := strings.Index(content[startIdx+len(start):], end)
	if relEnd < 0 {
		return "", false
	}
	endIdx := startIdx + len(start) + relEnd + len(end)
	return content[startIdx:endIdx], true
}

// UpsertBlock inserts or replaces a marked HTML-comment block inside content.
// If <!-- markerID --> ... <!-- /markerID --> already exists, it is replaced
// in-place while preserving surrounding content. Otherwise, the block is
// appended separated by a single blank line.
func UpsertBlock(content, markerID, blockContent string) string {
	normalizedBlock := EnsureTags(markerID, blockContent)
	start := StartTag(markerID)
	end := EndTag(markerID)

	startIdx := strings.Index(content, start)
	if startIdx >= 0 {
		relEnd := strings.Index(content[startIdx+len(start):], end)
		if relEnd >= 0 {
			endIdx := startIdx + len(start) + relEnd + len(end)
			updated := content[:startIdx] + normalizedBlock + content[endIdx:]
			return normalizeNewlines(updated)
		}
	}

	trimmedExisting := strings.TrimRight(content, " \t\r\n")
	if trimmedExisting == "" {
		return normalizedBlock + "\n"
	}
	return normalizeNewlines(trimmedExisting + "\n\n" + normalizedBlock + "\n")
}

// RemoveBlock removes <!-- markerID --> ... <!-- /markerID --> from content.
// It returns the cleaned content and true if a block was found and removed,
// or the original content and false if the block was not present.
func RemoveBlock(content, markerID string) (string, bool) {
	start := StartTag(markerID)
	end := EndTag(markerID)

	startIdx := strings.Index(content, start)
	if startIdx < 0 {
		return content, false
	}
	relEnd := strings.Index(content[startIdx+len(start):], end)
	if relEnd < 0 {
		return content, false
	}
	endIdx := startIdx + len(start) + relEnd + len(end)

	// Consume one trailing newline if present so removal does not leave extra blank lines.
	if endIdx < len(content) && content[endIdx] == '\r' {
		endIdx++
	}
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	updated := content[:startIdx] + content[endIdx:]
	cleaned := normalizeNewlines(updated)
	if strings.TrimSpace(cleaned) == "" {
		return "", true
	}
	return cleaned, true
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return ""
	}
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	// Collapse trailing multiple newlines into a single newline.
	s = strings.TrimRight(s, "\n") + "\n"
	// Collapse leading blank lines while preserving internal paragraph breaks.
	s = strings.TrimLeft(s, "\n")
	return s
}
