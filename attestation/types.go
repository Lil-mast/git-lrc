package attestation

import "time"

// ReviewSession represents a single review iteration stored in the DB.
type ReviewSession struct {
	ID        int64     `json:"id"`
	TreeHash  string    `json:"tree_hash"`
	Branch    string    `json:"branch"`
	Action    string    `json:"action"`
	Timestamp time.Time `json:"timestamp"`
	DiffFiles string    `json:"diff_files"`
	ReviewID  string    `json:"review_id"`
}

// FileEntry is a slim representation of a file diff for storage.
type FileEntry struct {
	FilePath string      `json:"file_path"`
	Hunks    []HunkRange `json:"hunks"`
}

// HunkRange stores line-range info from a hunk.
type HunkRange struct {
	OldStartLine int `json:"old_start_line"`
	OldLineCount int `json:"old_line_count"`
	NewStartLine int `json:"new_start_line"`
	NewLineCount int `json:"new_line_count"`
}

// CoverageResult holds computed coverage statistics.
type CoverageResult struct {
	Iterations       int     `json:"iterations"`
	PriorAICovPct    float64 `json:"prior_ai_coverage_pct"`
	CoveredLines     int     `json:"covered_lines"`
	TotalLines       int     `json:"total_lines"`
	PriorReviewCount int     `json:"prior_review_count"`
}
