package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
)

type decisionExecutionContext struct {
	precommit          bool
	verbose            bool
	initialMsg         string
	commitMsgPath      string
	diffContent        []byte
	reviewID           string
	attestationWritten *bool
}

func normalizeDecisionCode(code int) int {
	return code
}

func precommitExitCodeForDecision(code int) int {
	code = normalizeDecisionCode(code)
	if code == decisionVouch {
		return decisionSkip
	}
	return code
}

func executeDecision(code int, message string, push bool, ctx decisionExecutionContext) error {
	code = normalizeDecisionCode(code)
	switch code {
	case decisionAbort:
		syncedPrintln("\n❌ Commit aborted by user")
		return cli.Exit("", decisionAbort)
	case decisionCommit:
		if ctx.precommit {
			syncedPrintln("\n✅ Proceeding with commit")
		}
		finalMsg := strings.TrimSpace(message)
		if finalMsg == "" {
			finalMsg = strings.TrimSpace(ctx.initialMsg)
		}
		if ctx.precommit {
			if ctx.commitMsgPath != "" {
				if strings.TrimSpace(finalMsg) != "" {
					if err := persistCommitMessage(ctx.commitMsgPath, finalMsg); err != nil {
						syncedFprintf(os.Stderr, "Warning: failed to store commit message: %v\n", err)
					}
				} else {
					_ = clearCommitMessageFile(ctx.commitMsgPath)
				}
			}

			if push {
				if err := persistPushRequest(ctx.commitMsgPath); err != nil {
					syncedFprintf(os.Stderr, "Warning: failed to store push request: %v\n", err)
				}
			} else {
				_ = clearPushRequest(ctx.commitMsgPath)
			}

			return cli.Exit("", decisionCommit)
		}
		if err := runCommitAndMaybePush(finalMsg, push, ctx.verbose); err != nil {
			return err
		}
		return nil
	case decisionSkip:
		syncedPrintln("\n⏭️  Review skipped, proceeding with commit")
		if err := ensureAttestation("skipped", ctx.verbose, ctx.attestationWritten); err != nil {
			return err
		}
		if ctx.precommit {
			_ = clearCommitMessageFile(ctx.commitMsgPath)
			_ = clearPushRequest(ctx.commitMsgPath)
			return cli.Exit("", decisionSkip)
		}
		if err := runCommitAndMaybePush(strings.TrimSpace(message), push, ctx.verbose); err != nil {
			return err
		}
		return nil
	case decisionVouch:
		syncedPrintln("\n✅ Vouched, proceeding with commit")
		if err := recordCoverageAndAttest("vouched", ctx.diffContent, ctx.reviewID, ctx.verbose, ctx.attestationWritten); err != nil {
			return fmt.Errorf("vouch failed: %w", err)
		}
		if ctx.precommit {
			_ = clearCommitMessageFile(ctx.commitMsgPath)
			_ = clearPushRequest(ctx.commitMsgPath)
			return cli.Exit("", decisionSkip)
		}
		if err := runCommitAndMaybePush(strings.TrimSpace(message), push, ctx.verbose); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("invalid decision code: %d", code)
	}
}
