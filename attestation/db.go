package attestation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ReviewDBPath resolves the review DB location under .git/lrc/reviews.db.
func ReviewDBPath(resolveGitDir func() (string, error)) (string, error) {
	gitDir, err := resolveGitDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve git dir: %w", err)
	}
	if !filepath.IsAbs(gitDir) {
		gitDir, err = filepath.Abs(gitDir)
		if err != nil {
			return "", fmt.Errorf("failed to absolutize git dir: %w", err)
		}
	}
	lrcDir := filepath.Join(gitDir, "lrc")
	if err := os.MkdirAll(lrcDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lrc directory: %w", err)
	}
	return filepath.Join(lrcDir, "reviews.db"), nil
}

// OpenSQLiteReviewDB opens and initializes the SQLite review DB.
func OpenSQLiteReviewDB(dbPath, schema string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open review database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize review database schema: %w", err)
	}

	return db, nil
}

// CurrentBranch returns current git branch name, or HEAD when detached.
func CurrentBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine branch (detached HEAD?): %v\n", err)
		return "HEAD"
	}
	return strings.TrimSpace(string(out))
}

// InsertReviewSession inserts a new review session row.
func InsertReviewSession(db *sql.DB, treeHash, branch, action string, files []FileEntry, reviewID string) error {
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return fmt.Errorf("failed to marshal diff files: %w", err)
	}

	_, err = db.Exec(
		`INSERT INTO review_sessions (tree_hash, branch, action, timestamp, diff_files, review_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		treeHash, branch, action, time.Now().UTC().Format(time.RFC3339), string(filesJSON), reviewID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert review session: %w", err)
	}
	return nil
}

// CountIterations returns total review session count for a branch.
func CountIterations(db *sql.DB, branch string) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM review_sessions WHERE branch = ?`, branch).Scan(&count)
	return count, err
}

// GetPriorReviewedSessions returns branch sessions with action=reviewed in timestamp order.
func GetPriorReviewedSessions(db *sql.DB, branch string) ([]ReviewSession, error) {
	rows, err := db.Query(
		`SELECT id, tree_hash, branch, action, timestamp, diff_files, review_id
		 FROM review_sessions
		 WHERE branch = ? AND action = 'reviewed'
		 ORDER BY timestamp ASC`,
		branch,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []ReviewSession
	for rows.Next() {
		var s ReviewSession
		var ts, diffFiles, reviewID string
		if err := rows.Scan(&s.ID, &s.TreeHash, &s.Branch, &s.Action, &ts, &diffFiles, &reviewID); err != nil {
			return nil, err
		}
		parsedTime, parseErr := time.Parse(time.RFC3339, ts)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: malformed timestamp %q in review session %d: %v\n", ts, s.ID, parseErr)
		}
		s.Timestamp = parsedTime
		s.DiffFiles = diffFiles
		s.ReviewID = reviewID
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// CleanupReviewSessions deletes all sessions for a branch.
func CleanupReviewSessions(db *sql.DB, branch string) (int64, error) {
	result, err := db.Exec(`DELETE FROM review_sessions WHERE branch = ?`, branch)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CleanupAllSessions deletes all sessions from the review database.
func CleanupAllSessions(db *sql.DB) (int64, error) {
	result, err := db.Exec(`DELETE FROM review_sessions`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
