# Simulator Module

## Purpose and Rationale

The simulator module exists to validate the full interactive review and commit flow in a deterministic, automated, and cross-platform-safe way before changing production orchestration.

This module was added to solve these problems:

- Running real AI reviews repeatedly is slow, costly, and noisy for debugging.
- Manual browser actions are hard to reproduce and easy to miss in edge cases.
- Terminal and web decisions race in real usage; that behavior must be tested deterministically.
- Hook-related side effects (commit message override and push marker files) need automated verification.

### Design Goals

- No real AI review API calls in simulator tests.
- No manual browser interaction.
- Deterministic event injection and repeatable outcomes.
- Exhaustive case coverage for trigger modes, decisions, phases, message sources, and race conditions.
- Automated assertions for decision outcomes, exit behavior, message resolution, and precommit artifacts.

## What Is Simulated

- Terminal decisions (skip, vouch, abort, commit)
- Web decisions (/commit, /commit-push, /skip, /vouch, /abort)
- Signal-like abort events
- Poll-to-complete transition and phase changes
- First-valid-decision-wins race behavior
- Precommit artifact side effects in temporary .git directories:
  - livereview_commit_message
  - livereview_push_request

## Core Rules Enforced

- Running phase allows: skip, vouch, abort.
- Complete phase allows: commit, abort.
- Abort is always terminal.
- First valid decision wins; late decisions are ignored.
- Web commit requires non-empty message.
- Precommit vouch maps to skip exit code behavior.

## Case Table (Triggers x Decisions)

| Case ID | Trigger | Message Input Context | Decision | Expected Behavior |
|---|---|---|---|---|
| C01 | git commit -m "msg" | CLI message already present | Commit | Commit proceeds; uses CLI message unless explicitly overridden by Web UI submit. |
| C02 | git commit -m "msg" | CLI message already present | Skip | Commit proceeds with skip semantics; skip attestation/action recorded. |
| C03 | git commit -m "msg" | CLI message already present | Vouch | Commit proceeds with vouch semantics; vouch attestation/action recorded. |
| C04 | git commit -m "msg" | CLI message already present | Abort | Commit is blocked/aborted. |
| C05 | git commit (no -m) | Message entered later in editor | Commit | Commit proceeds; editor-later message is used as final message path. |
| C06 | git commit (no -m) | Message entered later in editor | Skip | Skip decision proceeds; final message flow still resolves correctly through editor path. |
| C07 | git commit (no -m) | Message entered later in editor | Vouch | Vouch decision proceeds; final message flow still resolves correctly through editor path. |
| C08 | git commit (no -m) | Message entered later in editor | Abort | Commit is blocked before finalize. |
| C09 | git lrc | No message or runtime default | Skip | Allowed only while review is running; proceed with skip semantics. |
| C10 | git lrc | No message or runtime default | Vouch | Allowed only while review is running; proceed with vouch semantics. |
| C11 | git lrc | Any | Abort | Always valid; stop workflow. |
| C12 | git lrc | CLI/Web prefill possible | Commit | Valid after review complete; commit proceeds using resolved message source. |
| C13 | lrc | No message or runtime default | Skip | Same behavioral contract as git lrc skip. |
| C14 | lrc | No message or runtime default | Vouch | Same behavioral contract as git lrc vouch. |
| C15 | lrc | Any | Abort | Same behavioral contract as git lrc abort. |
| C16 | lrc | CLI/Web prefill possible | Commit | Commit after completion, with proper final message resolution. |
| C17 | Serve/Web UI mode | CLI prefill exists | Web Commit | Web message should populate/override final message in commit flow. |
| C18 | Serve/Web UI mode | CLI prefill exists | Web Commit-Push | Commit plus push signaling behavior must be preserved. |
| C19 | Serve/Web UI mode | No CLI message | Web Commit | Web message becomes final commit message. |
| C20 | Serve/Web UI mode | Any | Web Abort | Abort path terminates and blocks commit. |
| C21 | Serve + Terminal concurrent | Mixed | Terminal wins race | First valid decision wins; later channel inputs ignored. |
| C22 | Serve + Terminal concurrent | Mixed | Web wins race | First valid decision wins; terminal late inputs ignored. |
| C23 | Poll completes first | Mixed | Late user action | Grace-window behavior applies; decision in grace window can still win. |
| C24 | Any | Any | Invalid phase action | Reject invalid action (phase gate), keep workflow alive for valid action. |

## Message Resolution Table (Formal)

| Case ID | CLI Message | Web Message | Editor-Later Message | Expected Final Message |
|---|---|---|---|---|
| M01 | present | empty | none | CLI message |
| M02 | present | present | none | Web message |
| M03 | empty | present | none | Web message |
| M04 | present | present | present | Must be locked by parity test (no assumptions until tested) |
| M05 | empty | empty | present | Editor message |
| M06 | present | empty | present | Must be locked by parity test (override order validated) |

## Formal Rules

1. Running phase: Skip, Vouch, Abort.
2. Complete phase: Commit, Abort.
3. Abort always terminal.
4. First valid decision wins across terminal/web/signal races.
5. Final message source is asserted per scenario, not guessed.
6. Every case must assert side effects too: commit created or blocked, push behavior, attestation behavior.

## HTTP Action Validation Covered

- POST /commit
- POST /commit-push
- POST /skip
- POST /vouch
- POST /abort
- Method not allowed handling
- Unknown path handling
- Invalid phase action conflict handling
- Empty web commit message rejection
- Message sanitization behavior

## How to Run

From repository root:

- go test ./internal/simulator

Recommended compatibility spot-check:

- go test ./internal/appcore -run 'TestActionAllowedInPhase|TestValidateInteractiveDecisionRequest|TestReadCommitMessageFromRequest|TestPollReviewFakeCompletes|TestPollReviewFakeCancelled'

## Notes for Future Additions

When adding new workflow behavior:

1. Add/extend simulator scenario(s) first.
2. Add explicit expected outcome assertions (decision, message, exit, artifacts).
3. Keep network calls fake-only in simulator tests.
4. Keep browser interaction simulated through in-process HTTP handlers.
5. Add timeline assertions for new race-sensitive behavior.
