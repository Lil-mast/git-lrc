package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ValidateGeminiKey checks the key against LiveReview's validate-key endpoint.
func ValidateGeminiKey(result *SetupResult, geminiKey string) (bool, string, error) {
	reqBody := ValidateKeyRequest{
		Provider: "gemini",
		APIKey:   geminiKey,
		Model:    DefaultGeminiModel,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return false, "", err
	}

	req, err := http.NewRequest("POST", CloudAPIURL+"/api/v1/aiconnectors/validate-key",
		bytes.NewReader(bodyJSON))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+result.AccessToken)
	req.Header.Set("X-Org-Context", result.OrgID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("failed to validate key: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read validation response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("validate-key returned %d: %s", resp.StatusCode, string(body))
	}

	var valResp ValidateKeyResponse
	if err := json.Unmarshal(body, &valResp); err != nil {
		return false, "", fmt.Errorf("failed to parse validation response: %w", err)
	}

	return valResp.Valid, valResp.Message, nil
}

// CreateGeminiConnector creates a Gemini AI connector in LiveReview.
func CreateGeminiConnector(result *SetupResult, geminiKey string) error {
	reqBody := CreateConnectorRequest{
		ProviderName:  "gemini",
		APIKey:        geminiKey,
		ConnectorName: "Gemini Flash",
		SelectedModel: DefaultGeminiModel,
		DisplayOrder:  0,
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", CloudAPIURL+"/api/v1/aiconnectors",
		bytes.NewReader(bodyJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+result.AccessToken)
	req.Header.Set("X-Org-Context", result.OrgID)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read connector response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create connector returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
