package attestation

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type LineRange struct {
	Start int
	End   int
}

func CountTotalNewLines(files []FileEntry) int {
	total := 0
	for _, f := range files {
		for _, h := range f.Hunks {
			if h.NewLineCount > 0 {
				total += h.NewLineCount
			}
		}
	}
	return total
}

func MarkAllNewLines(covered map[string]bool, f FileEntry) {
	for _, h := range f.Hunks {
		for line := h.NewStartLine; line < h.NewStartLine+h.NewLineCount; line++ {
			covered[fmt.Sprintf("%s:%d", f.FilePath, line)] = true
		}
	}
}

func DiffTreeFiles(tree1, tree2 string) ([]string, error) {
	out, err := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", tree1, tree2).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree failed: %w", err)
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

func DiffTreeFileHunks(tree1, tree2, filePath string) ([]HunkRange, error) {
	out, err := exec.Command("git", "diff", tree1, tree2, "--", filePath).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s %s -- %s failed: %w", tree1, tree2, filePath, err)
	}
	return ParseHunkRangesFromDiff(string(out)), nil
}

func LineInRanges(line int, ranges []LineRange) bool {
	for _, r := range ranges {
		if line >= r.Start && line <= r.End {
			return true
		}
	}
	return false
}

func MarkOverlappingLines(covered map[string]bool, filePath string, currentHunks, priorHunks []HunkRange, priorTree, currentTree string) {
	interTreeDiff, err := DiffTreeFileHunks(priorTree, currentTree, filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not diff %s between trees %s..%s: %v\n", filePath, priorTree[:8], currentTree[:8], err)
		return
	}

	changedRanges := make([]LineRange, 0, len(interTreeDiff))
	for _, h := range interTreeDiff {
		changedRanges = append(changedRanges, LineRange{Start: h.NewStartLine, End: h.NewStartLine + h.NewLineCount - 1})
	}

	for _, h := range currentHunks {
		for line := h.NewStartLine; line < h.NewStartLine+h.NewLineCount; line++ {
			if !LineInRanges(line, changedRanges) {
				covered[fmt.Sprintf("%s:%d", filePath, line)] = true
			}
		}
	}
}
