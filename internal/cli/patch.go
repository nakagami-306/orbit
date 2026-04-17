package cli

import (
	"fmt"
	"strings"
)

// applyPatch applies an old→new text replacement on content.
// If old is empty, new is appended to the end.
// If new is empty, old is deleted.
// Returns an error if old is not found or matches multiple times.
func applyPatch(content, old, new string) (string, error) {
	if old == "" {
		// Append mode
		if content == "" {
			return new, nil
		}
		return content + "\n" + new, nil
	}

	count := strings.Count(content, old)
	if count == 0 {
		return "", fmt.Errorf("--old text not found in section content")
	}
	if count > 1 {
		return "", fmt.Errorf("--old text matches %d times in section content (must be unique)", count)
	}

	return strings.Replace(content, old, new, 1), nil
}
