package simulator

import (
	"encoding/json"
	"strings"

	"github.com/HexmosTech/git-lrc/internal/decisionflow"
)

const (
	HTTPStatusOK               = 200
	HTTPStatusBadRequest       = 400
	HTTPStatusNotFound         = 404
	HTTPStatusMethodNotAllowed = 405
	HTTPStatusConflict         = 409
)

// HandleWebDecision simulates an in-process web decision endpoint call.
// It returns the mapped decision event and HTTP-like status code.
func HandleWebDecision(phase decisionflow.Phase, method, path string, body []byte) (DecisionEvent, int) {
	if method != "POST" {
		return DecisionEvent{}, HTTPStatusMethodNotAllowed
	}

	var (
		code int
		push bool
	)

	switch path {
	case "/commit":
		code = decisionflow.DecisionCommit
	case "/commit-push":
		code = decisionflow.DecisionCommit
		push = true
	case "/skip":
		code = decisionflow.DecisionSkip
	case "/vouch":
		code = decisionflow.DecisionVouch
	case "/abort":
		code = decisionflow.DecisionAbort
	default:
		return DecisionEvent{}, HTTPStatusNotFound
	}

	msg := readCommitMessageFromJSON(body)
	if err := decisionflow.ValidateRequest(code, msg, phase); err != nil {
		if reqErr, ok := err.(*decisionflow.RequestError); ok {
			return DecisionEvent{}, reqErr.StatusCode()
		}
		return DecisionEvent{}, HTTPStatusBadRequest
	}

	return DecisionEvent{
		Source:  DecisionSourceWeb,
		Code:    code,
		Message: msg,
		Push:    push,
	}, HTTPStatusOK
}

func readCommitMessageFromJSON(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var payload struct {
		Message string `json:"message"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}

	msg := strings.TrimRight(payload.Message, "\r\n")
	msg = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, msg)

	return msg
}
