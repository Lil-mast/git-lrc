package appcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/HexmosTech/git-lrc/internal/reviewmodel"
	"github.com/HexmosTech/git-lrc/network"
	"github.com/urfave/cli/v2"
)

type quotaStatusResponse struct {
	PlanType          string                         `json:"plan_type"`
	DailyLimit        *int                           `json:"daily_limit,omitempty"`
	DailyUsed         int                            `json:"daily_used"`
	CanTriggerReviews bool                           `json:"can_trigger_reviews"`
	Envelope          *reviewmodel.PlanUsageEnvelope `json:"envelope,omitempty"`
}

func RunUsageInspect(c *cli.Context) error {
	verbose := c.Bool("verbose")
	apiURLOverride := strings.TrimSpace(c.String("api-url"))
	output := strings.TrimSpace(strings.ToLower(c.String("output")))
	if output == "" {
		output = "pretty"
	}
	if output != "pretty" && output != "json" {
		return fmt.Errorf("invalid output format: %s (must be pretty or json)", output)
	}

	config, err := loadConfigValues("", apiURLOverride, verbose)
	if err != nil {
		return err
	}
	if strings.TrimSpace(config.JWT) == "" || strings.TrimSpace(config.OrgID) == "" {
		return fmt.Errorf("usage inspect requires org_id and jwt in ~/.lrc.toml; run 'lrc ui' to login and select an organization")
	}

	url := strings.TrimSuffix(config.APIURL, "/") + "/api/v1/quota/status"
	client := network.NewReviewAPIClient(15 * time.Second)
	resp, err := network.ReviewForwardJSONWithBearer(client, http.MethodGet, url, nil, config.JWT, config.OrgID)
	if err != nil {
		return fmt.Errorf("failed to fetch quota status: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("quota status request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(resp.Body)))
	}

	var payload quotaStatusResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return fmt.Errorf("failed to parse quota status response: %w", err)
	}

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	fmt.Println("LiveReview Usage")
	fmt.Println(strings.Repeat("=", 48))
	fmt.Printf("plan_type: %s\n", strings.TrimSpace(payload.PlanType))
	if payload.DailyLimit != nil {
		fmt.Printf("daily_usage: %d/%d\n", payload.DailyUsed, *payload.DailyLimit)
	} else {
		fmt.Printf("daily_usage: %d/unlimited\n", payload.DailyUsed)
	}
	fmt.Printf("can_trigger_reviews: %t\n", payload.CanTriggerReviews)

	if payload.Envelope != nil {
		fmt.Println("\nenvelope:")
		for _, line := range formatEnvelopeUsageLines(payload.Envelope) {
			fmt.Printf("  - %s\n", line)
		}
	}

	return nil
}
