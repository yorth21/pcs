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
	normalized := strings.ReplaceAll(blockContent, "\r\n", "\n")
	trimmed := strings.TrimSpace(normalized)
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
// in-place while preserving all surrounding content byte-for-byte. Otherwise,
// the block is appended separated by a single blank line using the file's
// dominant line ending.
func UpsertBlock(content, markerID, blockContent string) string {
	eol := detectEOL(content)
	normalizedBlock := EnsureTags(markerID, blockContent)
	if eol == "\r\n" {
		normalizedBlock = strings.ReplaceAll(normalizedBlock, "\n", "\r\n")
	}

	start := StartTag(markerID)
	end := EndTag(markerID)

	startIdx := strings.Index(content, start)
	if startIdx >= 0 {
		relEnd := strings.Index(content[startIdx+len(start):], end)
		if relEnd >= 0 {
			endIdx := startIdx + len(start) + relEnd + len(end)
			return content[:startIdx] + normalizedBlock + content[endIdx:]
		}
	}

	if content == "" {
		return normalizedBlock + eol
	}
	if strings.HasSuffix(content, eol+eol) {
		return content + normalizedBlock + eol
	}
	if strings.HasSuffix(content, eol) {
		return content + eol + normalizedBlock + eol
	}
	return content + eol + eol + normalizedBlock + eol
}

// RemoveBlock removes <!-- markerID --> ... <!-- /markerID --> from content
// without altering any content outside the removed block and its adjacent
// blank-line separator.
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

	eol := detectEOL(content)
	prefix := content[:startIdx]
	suffix := content[endIdx:]

	// Case 1: Block is followed by a blank-line separator (e.g., first or middle block).
	if strings.HasPrefix(suffix, eol+eol) {
		if startIdx == 0 || strings.HasSuffix(prefix, eol+eol) {
			suffix = suffix[len(eol+eol):]
			return prefix + suffix, true
		}
	}

	// Consume one trailing line ending immediately following the closing tag.
	if strings.HasPrefix(suffix, "\r\n") {
		suffix = suffix[2:]
	} else if strings.HasPrefix(suffix, "\n") {
		suffix = suffix[1:]
	}

	// Case 2: Block was at the end of the file and preceded by a blank line added by UpsertBlock.
	if suffix == "" && strings.HasSuffix(prefix, eol+eol) {
		prefix = prefix[:len(prefix)-len(eol)]
	}

	return prefix + suffix, true
}

func detectEOL(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}
