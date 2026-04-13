package appui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HexmosTech/git-lrc/network"
	uicfg "github.com/HexmosTech/git-lrc/ui"
)

func TestHandleUsageChipMissingAuth(t *testing.T) {
	srv := &connectorManagerServer{
		cfg: &uiRuntimeConfig{
			APIURL: "https://example.com",
		},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Available {
		t.Fatalf("expected unavailable payload")
	}
	if !strings.Contains(strings.ToLower(payload.UnavailableReason), "not authenticated") {
		t.Fatalf("expected not authenticated reason, got %q", payload.UnavailableReason)
	}
}

func TestHandleUsageChipAggregatesData(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"envelope": map[string]interface{}{
					"plan_code":       "team_32usd",
					"usage_percent":   82,
					"blocked":         false,
					"loc_used_month":  int64(82000),
					"loc_limit_month": int64(100000),
					"reset_at":        "2026-04-30T00:00:00Z",
				},
			})
		case "/api/v1/billing/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"billing": map[string]interface{}{
					"current_plan_code":  "team_32usd",
					"billing_period_end": "2026-04-30T00:00:00Z",
					"loc_used_month":     int64(82000),
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
					"total_billable_loc":  int64(1200),
					"operation_count":     int64(8),
					"usage_share_percent": 4.2,
				},
			})
		case "/api/v1/billing/usage/members":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"members": []map[string]interface{}{
					{"actor_email": "a@example.com", "actor_kind": "user", "total_billable_loc": int64(4200), "usage_share_percent": 24.4},
					{"actor_email": "b@example.com", "actor_kind": "user", "total_billable_loc": int64(3000), "usage_share_percent": 17.1},
				},
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	srv := &connectorManagerServer{
		cfg: &uiRuntimeConfig{
			APIURL: backend.URL,
			JWT:    "jwt-token",
			OrgID:  "42",
		},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !payload.Available {
		t.Fatalf("expected available payload, got unavailable reason: %q", payload.UnavailableReason)
	}
	if payload.PlanCode != "team_32usd" {
		t.Fatalf("expected plan team_32usd, got %q", payload.PlanCode)
	}
	if payload.UsagePct != 82 {
		t.Fatalf("expected usage pct 82, got %d", payload.UsagePct)
	}
	if payload.LOCUsed != 82000 || payload.LOCLimit != 100000 {
		t.Fatalf("unexpected loc usage %d/%d", payload.LOCUsed, payload.LOCLimit)
	}
	if payload.MyUsageLOC != 1200 || payload.MyOperationCount != 8 {
		t.Fatalf("unexpected my usage values: loc=%d ops=%d", payload.MyUsageLOC, payload.MyOperationCount)
	}
	if !payload.CanViewTeamBreakdown {
		t.Fatalf("expected team breakdown visibility")
	}
	if len(payload.TopMembers) != 2 {
		t.Fatalf("expected 2 members, got %d", len(payload.TopMembers))
	}
}

func TestHandleUsageChipMembersForbiddenStillAvailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/quota/status":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"envelope": map[string]interface{}{
					"plan_code":       "free_30k",
					"usage_percent":   35,
					"blocked":         false,
					"loc_used_month":  int64(10500),
					"loc_limit_month": int64(30000),
				},
			})
		case "/api/v1/billing/status", "/api/v1/subscriptions/current", "/api/v1/billing/upgrade/request-status", "/api/v1/billing/usage/me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case "/api/v1/billing/usage/members":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"forbidden"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer backend.Close()

	srv := &connectorManagerServer{
		cfg:    &uiRuntimeConfig{APIURL: backend.URL, JWT: "jwt-token", OrgID: "99"},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !payload.Available {
		t.Fatalf("expected payload to stay available when members endpoint is forbidden")
	}
	if payload.CanViewTeamBreakdown {
		t.Fatalf("expected team breakdown visibility to be false")
	}
}

func TestHandleUsageChipFallsBackToQuotaPlanTypeWhenBillingUnavailable(t *testing.T) {
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

	srv := &connectorManagerServer{
		cfg:    &uiRuntimeConfig{APIURL: backend.URL, JWT: "jwt-token", OrgID: "77"},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

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

func TestHandleUsageChipPrefersBillingPlanWhenQuotaIsStale(t *testing.T) {
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

	srv := &connectorManagerServer{
		cfg:    &uiRuntimeConfig{APIURL: backend.URL, JWT: "jwt-token", OrgID: "3"},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.PlanCode != "loc_400k" {
		t.Fatalf("expected billing plan loc_400k, got %q", payload.PlanCode)
	}
	if payload.LOCLimit != 400000 {
		t.Fatalf("expected billing LOC limit 400000, got %d", payload.LOCLimit)
	}
}

func TestHandleUsageChipNoSignalMarksUnavailable(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer backend.Close()

	srv := &connectorManagerServer{
		cfg:    &uiRuntimeConfig{APIURL: backend.URL, JWT: "jwt-token", OrgID: "77"},
		client: network.NewUIConnectorClient(2 * time.Second),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ui/usage-chip", nil)
	resp := httptest.NewRecorder()
	srv.handleUsageChip(resp, req)

	var payload uicfg.UsageChipResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Available {
		t.Fatalf("expected payload unavailable when endpoints return no usage signal")
	}
	if !strings.Contains(strings.ToLower(payload.UnavailableReason), "unavailable") {
		t.Fatalf("expected generic unavailable reason, got %q", payload.UnavailableReason)
	}
}
