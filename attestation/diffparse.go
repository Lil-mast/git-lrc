package attestation

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// ParseHunkRangesFromDiff extracts hunk ranges from a unified diff string.
func ParseHunkRangesFromDiff(diffStr string) []HunkRange {
	re := regexp.MustCompile(`@@ -(\d+),?(\d*) \+(\d+),?(\d*) @@`)
	matches := re.FindAllStringSubmatch(diffStr, -1)
	if len(matches) == 0 {
		return nil
	}

	hunks := make([]HunkRange, 0, len(matches))
	for _, m := range matches {
		if len(m) < 5 {
			continue
		}

		oldStart, err := strconv.Atoi(m[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: malformed hunk old_start %q: %v\n", m[1], err)
			continue
		}
		oldCount := 1
		if m[2] != "" {
			parsed, err := strconv.Atoi(m[2])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: malformed hunk old_count %q: %v\n", m[2], err)
				continue
			}
			oldCount = parsed
		}

		newStart, err := strconv.Atoi(m[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: malformed hunk new_start %q: %v\n", m[3], err)
			continue
		}
		newCount := 1
		if m[4] != "" {
			parsed, err := strconv.Atoi(m[4])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: malformed hunk new_count %q: %v\n", m[4], err)
				continue
			}
			newCount = parsed
		}

		hunks = append(hunks, HunkRange{
			OldStartLine: oldStart,
			OldLineCount: oldCount,
			NewStartLine: newStart,
			NewLineCount: newCount,
		})
	}

	return hunks
}
