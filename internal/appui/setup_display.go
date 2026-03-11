package appui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	cReset  = "\033[0m"
	cBold   = "\033[1m"
	cDim    = "\033[2m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
	cRed    = "\033[31m"
	cCyan   = "\033[36m"
	cBlue   = "\033[34m"
)

var setupColors = true

var (
	buildVersion = "unknown"
	buildTime    = "unknown"
	buildCommit  = "unknown"
)

func SetBuildInfo(version, builtAt, commit string) {
	if strings.TrimSpace(version) != "" {
		buildVersion = version
	}
	if strings.TrimSpace(builtAt) != "" {
		buildTime = builtAt
	}
	if strings.TrimSpace(commit) != "" {
		buildCommit = commit
	}
}

// colorsEnabled reports whether the terminal supports ANSI colors.
func colorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if runtime.GOOS == "windows" {
		if os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") != "" {
			return true
		}
		return false
	}
	return true
}

func init() {
	if !colorsEnabled() {
		setupColors = false
	}
}

func clr(code string) string {
	if setupColors {
		return code
	}
	return ""
}

func hyperlink(linkURL, text string) string {
	if !setupColors {
		return text + " (" + linkURL + ")"
	}
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", linkURL, text)
}

// setupLog captures debug output during setup for issue reporting.
type setupLog struct {
	entries []string
	logFile string
}

func newSetupLog() *setupLog {
	logFile := ""
	if homeDir, err := os.UserHomeDir(); err == nil {
		logFile = filepath.Join(homeDir, ".lrc-setup.log")
	} else {
		logFile = filepath.Join(os.TempDir(), "lrc-setup.log")
	}
	sl := &setupLog{logFile: logFile}
	sl.write("=== lrc setup started at %s ===", time.Now().Format(time.RFC3339))
	sl.write("lrc version: %s  build: %s  commit: %s", buildVersion, buildTime, buildCommit)
	sl.write("os: %s/%s", runtime.GOOS, runtime.GOARCH)
	return sl
}

func (sl *setupLog) write(format string, args ...interface{}) {
	entry := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
	sl.entries = append(sl.entries, entry)
}

func (sl *setupLog) flush() {
	content := strings.Join(sl.entries, "\n") + "\n"
	if err := os.WriteFile(sl.logFile, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not write debug log to %s: %v\n", sl.logFile, err)
	}
}

func (sl *setupLog) buildIssueURL(errMsg string) string {
	logContent := strings.Join(sl.entries, "\n")
	const maxLogLen = 4000
	if len(logContent) > maxLogLen {
		logContent = logContent[len(logContent)-maxLogLen:]
		logContent = "...(truncated)\n" + logContent
	}

	body := fmt.Sprintf("## `lrc setup` failed\n\n**Error:** `%s`\n\n**Version:** %s (%s, %s)\n**OS:** %s/%s\n\n<details>\n<summary>Debug log</summary>\n\n```\n%s\n```\n</details>\n",
		errMsg, buildVersion, buildTime, buildCommit, runtime.GOOS, runtime.GOARCH, logContent)

	params := url.Values{}
	params.Set("title", "lrc setup: "+errMsg)
	params.Set("body", body)
	params.Set("labels", "bug,setup")

	return issuesURL + "?" + params.Encode()
}

func setupError(slog *setupLog, err error) error {
	errMsg := err.Error()
	slog.write("ERROR: %s", errMsg)
	slog.flush()

	fmt.Println()
	fmt.Printf("  %s%s❌ Setup failed%s\n", clr(cBold), clr(cRed), clr(cReset))
	fmt.Printf("  %s%s%s\n", clr(cRed), errMsg, clr(cReset))
	fmt.Println()
	fmt.Printf("  %sDebug log saved to:%s %s%s%s\n", clr(cDim), clr(cReset), clr(cYellow), slog.logFile, clr(cReset))
	fmt.Println()

	issueURL := slog.buildIssueURL(errMsg)
	fmt.Printf("  %s🐛 Report this issue:%s\n", clr(cBold), clr(cReset))
	fmt.Printf("     %s\n", hyperlink(issueURL, clr(cCyan)+issuesURL+clr(cReset)))
	fmt.Println()
	fmt.Printf("  %s(The link above pre-fills the issue with your debug log)%s\n", clr(cDim), clr(cReset))
	fmt.Println()

	return err
}

func printSetupSuccess(result *setupResult) {
	keyPreview := result.PlainAPIKey
	if len(keyPreview) > 16 {
		keyPreview = keyPreview[:16] + "..."
	}

	fmt.Println()
	fmt.Printf("  %s%s🎉 Setup Complete!%s\n", clr(cBold), clr(cGreen), clr(cReset))
	fmt.Printf("  %s─────────────────────────%s\n", clr(cDim), clr(cReset))
	fmt.Println()
	fmt.Printf("  %s📧 Email:%s    %s\n", clr(cBold), clr(cReset), result.Email)
	if result.OrgName != "" {
		fmt.Printf("  %s🏢 Org:%s      %s\n", clr(cBold), clr(cReset), result.OrgName)
	}
	fmt.Printf("  %s🔑 API Key:%s  %s%s%s\n", clr(cBold), clr(cReset), clr(cYellow), keyPreview, clr(cReset))
	fmt.Printf("  %s🤖 AI:%s       Gemini connector %s(%s)%s\n", clr(cBold), clr(cReset), clr(cDim), defaultGeminiModel, clr(cReset))
	fmt.Printf("  %s📁 Config:%s   %s~/.lrc.toml%s\n", clr(cBold), clr(cReset), clr(cCyan), clr(cReset))
	fmt.Println()
	fmt.Printf("  %sIn a git repo with staged changes:%s\n", clr(cDim), clr(cReset))
	fmt.Println()
	fmt.Printf("    %s$ %sgit add .%s\n", clr(cDim), clr(cReset), clr(cReset))
	fmt.Printf("    %s$ %sgit lrc review%s        %s# AI-powered code review%s\n", clr(cDim), clr(cGreen), clr(cReset), clr(cDim), clr(cReset))
	fmt.Printf("    %s$ %sgit lrc review --vouch%s %s# mark as manually reviewed%s\n", clr(cDim), clr(cGreen), clr(cReset), clr(cDim), clr(cReset))
	fmt.Printf("    %s$ %sgit lrc review --skip%s  %s# skip review for this change%s\n", clr(cDim), clr(cGreen), clr(cReset), clr(cDim), clr(cReset))
	fmt.Println()
}
