package appcore

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/HexmosTech/git-lrc/internal/naming"
	"github.com/HexmosTech/git-lrc/internal/reviewapi"
	"github.com/HexmosTech/git-lrc/internal/reviewmodel"
	"github.com/HexmosTech/git-lrc/internal/reviewopts"
	reviewpkg "github.com/HexmosTech/git-lrc/review"
	"github.com/urfave/cli/v2"
)

const (
	commitMessageFile   = "livereview_commit_message"
	editorWrapperScript = "lrc_editor.sh"
	editorBackupFile    = ".lrc_editor_backup"
	pushRequestFile     = "livereview_push_request"
)

const (
	lrcMarkerBegin        = "# BEGIN lrc managed section - DO NOT EDIT"
	lrcMarkerEnd          = "# END lrc managed section"
	defaultGlobalHooksDir = ".git-hooks"
	hooksMetaFilename     = ".lrc-hooks-meta.json"
)

var managedHooks = []string{"pre-commit", "prepare-commit-msg", "commit-msg", "post-commit"}

var (
	version    = "unknown"
	reviewMode = "prod"

	currentReviewState *ReviewState
	reviewStateMu      sync.RWMutex
)

func Configure(versionValue, reviewModeValue string) {
	if strings.TrimSpace(versionValue) != "" {
		version = versionValue
	}
	if strings.TrimSpace(reviewModeValue) != "" {
		reviewMode = reviewModeValue
	}
}

func isFakeReviewBuild() bool {
	return reviewpkg.IsFakeReviewBuild(reviewMode)
}

func fakeReviewWaitDuration() (time.Duration, error) {
	return reviewpkg.FakeReviewWaitDuration(os.Getenv("LRC_FAKE_REVIEW_WAIT"))
}

func buildFakeSubmitResponse() reviewmodel.DiffReviewCreateResponse {
	resp := reviewpkg.BuildFakeSubmitResponse(time.Now(), naming.GenerateFriendlyName())
	return reviewmodel.DiffReviewCreateResponse{
		ReviewID:     resp.ReviewID,
		Status:       resp.Status,
		FriendlyName: resp.FriendlyName,
	}
}

func buildFakeCompletedResult() *reviewmodel.DiffReviewResponse {
	resp := reviewpkg.BuildFakeCompletedResult()
	return &reviewmodel.DiffReviewResponse{
		Status:  resp.Status,
		Summary: resp.Summary,
		Files:   []reviewmodel.DiffReviewFileResult{},
	}
}

func pollReviewFake(reviewID string, pollInterval, wait time.Duration, verbose bool, cancel <-chan struct{}) (*reviewmodel.DiffReviewResponse, error) {
	if pollInterval <= 0 {
		pollInterval = 1 * time.Second
	}

	start := time.Now()
	deadline := start.Add(wait)
	fmt.Printf("Waiting for fake review completion (poll every %s, delay %s)...\r\n", pollInterval, wait)
	syncFileSafely(os.Stdout)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		now := time.Now()
		if !now.Before(deadline) {
			statusLine := fmt.Sprintf("Status: completed | elapsed: %s", now.Sub(start).Truncate(time.Second))
			fmt.Printf("\r%-80s\r\n", statusLine)
			syncFileSafely(os.Stdout)
			if verbose {
				log.Printf("fake review %s completed", reviewID)
			}
			return buildFakeCompletedResult(), nil
		}

		statusLine := fmt.Sprintf("Status: in_progress | elapsed: %s", now.Sub(start).Truncate(time.Second))
		fmt.Printf("\r%-80s", statusLine)
		syncFileSafely(os.Stdout)
		if verbose {
			log.Printf("fake review %s: %s", reviewID, statusLine)
		}

		select {
		case <-cancel:
			fmt.Printf("\r\n")
			syncFileSafely(os.Stdout)
			return nil, reviewapi.ErrPollCancelled
		case <-ticker.C:
		}
	}
}

func highlightURL(url string) string {
	return "\033[36m" + url + "\033[0m"
}

func buildReviewURL(apiURL, reviewID string) string {
	base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(apiURL, "/"), "/api"), "/api/v1")
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/#/reviews/%s", base, reviewID)
}

func pickServePort(preferredPort, maxTries int) (net.Listener, int, error) {
	for i := 0; i < maxTries; i++ {
		candidate := preferredPort + i

		if runtime.GOOS == "windows" {
			lnLocal, errLocal := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", candidate))
			lnAll, errAll := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidate))

			if errLocal != nil || errAll != nil {
				if lnLocal != nil {
					lnLocal.Close()
				}
				if lnAll != nil {
					lnAll.Close()
				}
				continue
			}

			lnAll.Close()
			return lnLocal, candidate, nil
		}

		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", candidate))
		if err == nil {
			return ln, candidate, nil
		}
	}

	return nil, 0, fmt.Errorf("no available port found starting from %d", preferredPort)
}

func RunReviewWithOptions(opts reviewopts.Options) error {
	return runReviewWithOptions(opts)
}

func RunHooksInstall(c *cli.Context) error {
	return runHooksInstall(c)
}

func RunHooksUninstall(c *cli.Context) error {
	return runHooksUninstall(c)
}

func RunHooksEnable(c *cli.Context) error {
	return runHooksEnable(c)
}

func RunHooksDisable(c *cli.Context) error {
	return runHooksDisable(c)
}

func RunHooksStatus(c *cli.Context) error {
	return runHooksStatus(c)
}

func RunAttestationTrailer(c *cli.Context) error {
	return runAttestationTrailer(c)
}
