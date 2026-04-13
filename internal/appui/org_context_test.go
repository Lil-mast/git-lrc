package appui

import (
	"strings"
	"testing"
)

func TestParseOrgIDFromBody(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		expected  int64
		errSubstr string
	}{
		{name: "numeric org id", body: `{"org_id": 42}`, expected: 42},
		{name: "string org id", body: `{"org_id": " 42 "}`, expected: 42},
		{name: "missing org id", body: `{}`, errSubstr: "org_id is required"},
		{name: "null org id", body: `{"org_id": null}`, errSubstr: "org_id is required"},
		{name: "fractional org id", body: `{"org_id": 42.5}`, errSubstr: "positive integer"},
		{name: "object org id", body: `{"org_id": {"value": 42}}`, errSubstr: "string or number"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orgID, err := parseOrgIDFromBody([]byte(tc.body))
			if tc.errSubstr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tc.errSubstr)
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errSubstr)) {
					t.Fatalf("expected error containing %q, got %v", tc.errSubstr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if orgID != tc.expected {
				t.Fatalf("expected org id %d, got %d", tc.expected, orgID)
			}
		})
	}
}
