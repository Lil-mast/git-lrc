package appui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	cfg "github.com/HexmosTech/git-lrc/config"
)

func decodeJWTClaims(jwt string) map[string]string {
	claims := map[string]string{}
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return claims
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims
	}

	parsed := map[string]interface{}{}
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return claims
	}

	for key, value := range parsed {
		if text, ok := value.(string); ok {
			claims[key] = strings.TrimSpace(text)
		}
	}

	return claims
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildLiveReviewURL(baseURL, apiPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	base = strings.TrimSuffix(base, "/api/v1")
	base = strings.TrimSuffix(base, "/api")
	return base + apiPath
}

func writeRawJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := io.Copy(w, bytes.NewReader(body)); err != nil {
		log.Printf("failed to write JSON response (status=%d): %v", status, err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeRawJSON(w, status, []byte(fmt.Sprintf(`{"error":%q}`, message)))
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to encode response")
		return
	}
	writeRawJSON(w, status, body)
}

func upsertQuotedConfigValue(content string, key string, value string) string {
	return cfg.UpsertQuotedConfigValue(content, key, value)
}

func stripManagedAIConnectorsSection(content string) string {
	return cfg.StripManagedAIConnectorsSection(content, aiConnectorsSectionBegin, aiConnectorsSectionEnd)
}

func renderManagedAIConnectorsSection(connectors []aiConnectorRemote) string {
	return cfg.RenderManagedAIConnectorsSection(connectors, aiConnectorsSectionBegin, aiConnectorsSectionEnd)
}
