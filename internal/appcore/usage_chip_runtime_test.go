package appcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildRuntimeUsageChipPayloadMissingCredentials(t *testing.T) {
	cfg := &Config{APIURL: "https://example.com"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if payload.Available {
		t.Fatalf("expected unavailable payload when jwt/org is missing")
	}
	if !strings.Contains(strings.ToLower(payload.UnavailableReason), "not authenticated") {
		t.Fatalf("unexpected unavailable reason: %q", payload.UnavailableReason)
	}
}

func TestBuildRuntimeUsageChipPayloadAggregatesData(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"envelope": map[string]interface{}{
					"plan_code":       "team_32usd",
					"usage_percent":   61,
					"blocked":         false,
					"loc_used_month":  int64(61000),
					"loc_limit_month": int64(100000),
					"reset_at":        "2026-04-30T00:00:00Z",
				},
			})
		case "/api/v1/billing/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"billing": map[string]interface{}{
					"current_plan_code":  "team_32usd",
					"billing_period_end": "2026-04-30T00:00:00Z",
					"loc_used_month":     int64(61000),
				},
				"available_plans": []map[string]interface{}{
					{"plan_code": "team_32usd", "monthly_loc_limit": int64(100000)},
				},
			})
		case "/api/v1/billing/upgrade/request-status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"request": map[string]interface{}{"customer_state": "none"},
			})
		case "/api/v1/subscriptions/current":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{})
		case "/api/v1/billing/usage/me":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"member": map[string]interface{}{
					"total_billable_loc":  int64(1900),
					"operation_count":     int64(14),
					"usage_share_percent": 7.5,
				},
			})
		case "/api/v1/billing/usage/members":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"members": []map[string]interface{}{
					{"actor_email": "one@example.com", "actor_kind": "user", "total_billable_loc": int64(9800), "usage_share_percent": 12.3},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	cfg := &Config{APIURL: backend.URL, JWT: "jwt-token", OrgID: "41"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if !payload.Available {
		t.Fatalf("expected available payload, got reason: %q", payload.UnavailableReason)
	}
	if payload.PlanCode != "team_32usd" {
		t.Fatalf("unexpected plan code: %q", payload.PlanCode)
	}
	if payload.UsagePct != 61 {
		t.Fatalf("expected usage pct 61, got %d", payload.UsagePct)
	}
	if payload.MyOperationCount != 14 {
		t.Fatalf("expected my operation count 14, got %d", payload.MyOperationCount)
	}
	if len(payload.TopMembers) != 1 {
		t.Fatalf("expected one top member, got %d", len(payload.TopMembers))
	}
}

func TestBuildRuntimeUsageChipPayloadMembersForbiddenStillAvailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"envelope": map[string]interface{}{
					"plan_code":       "free_30k",
					"usage_percent":   20,
					"blocked":         false,
					"loc_used_month":  int64(6000),
					"loc_limit_month": int64(30000),
				},
			})
		case "/api/v1/billing/status", "/api/v1/subscriptions/current", "/api/v1/billing/upgrade/request-status", "/api/v1/billing/usage/me":
			_, _ = w.Write([]byte(`{}`))
		case "/api/v1/billing/usage/members":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	cfg := &Config{APIURL: backend.URL, JWT: "jwt-token", OrgID: "41"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if !payload.Available {
		t.Fatalf("expected available payload")
	}
	if payload.CanViewTeamBreakdown {
		t.Fatalf("expected team breakdown to be hidden")
	}
}

func TestBuildRuntimeUsageChipPayloadFallsBackToQuotaPlanTypeWhenBillingUnavailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"plan_type": "free",
			})
		case "/api/v1/billing/status":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"Not Found"}`))
		case "/api/v1/subscriptions/current":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"plan_type": "free",
			})
		case "/api/v1/billing/upgrade/request-status", "/api/v1/billing/usage/me", "/api/v1/billing/usage/members":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	cfg := &Config{APIURL: backend.URL, JWT: "jwt-token", OrgID: "11"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if !payload.Available {
		t.Fatalf("expected payload available via quota/subscription fallback")
	}
	if payload.PlanCode != "free_30k" {
		t.Fatalf("expected normalized free plan code, got %q", payload.PlanCode)
	}
	if payload.LOCLimit != 30000 {
		t.Fatalf("expected free fallback LOC limit 30000, got %d", payload.LOCLimit)
	}
}

func TestBuildRuntimeUsageChipPayloadPrefersBillingPlanWhenQuotaIsStale(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"plan_type": "free_30k",
				"envelope": map[string]interface{}{
					"plan_code":       "free_30k",
					"usage_percent":   0,
					"blocked":         false,
					"loc_used_month":  int64(0),
					"loc_limit_month": int64(30000),
				},
			})
		case "/api/v1/billing/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"billing": map[string]interface{}{
					"current_plan_code":  "loc_400k",
					"billing_period_end": "2026-05-07T18:30:00Z",
					"loc_used_month":     int64(0),
				},
				"available_plans": []map[string]interface{}{
					{"plan_code": "free_30k", "monthly_loc_limit": int64(30000)},
					{"plan_code": "loc_400k", "monthly_loc_limit": int64(400000)},
				},
			})
		case "/api/v1/subscriptions/current", "/api/v1/billing/upgrade/request-status", "/api/v1/billing/usage/me", "/api/v1/billing/usage/members":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	cfg := &Config{APIURL: backend.URL, JWT: "jwt-token", OrgID: "3"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if payload.PlanCode != "loc_400k" {
		t.Fatalf("expected billing plan loc_400k, got %q", payload.PlanCode)
	}
	if payload.LOCLimit != 400000 {
		t.Fatalf("expected billing LOC limit 400000, got %d", payload.LOCLimit)
	}
}

func TestBuildRuntimeUsageChipPayloadNoSignalMarksUnavailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	cfg := &Config{APIURL: backend.URL, JWT: "jwt-token", OrgID: "55"}
	payload := buildRuntimeUsageChipPayload(cfg, false)

	if payload.Available {
		t.Fatalf("expected payload unavailable when endpoints return no usage signal")
	}
	if !strings.Contains(strings.ToLower(payload.UnavailableReason), "unavailable") {
		t.Fatalf("expected generic unavailable reason, got %q", payload.UnavailableReason)
	}
}
