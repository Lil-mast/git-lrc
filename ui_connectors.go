package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/urfave/cli/v2"
)

const defaultUIPort = 8090

const (
	aiConnectorsSectionBegin = "# BEGIN lrc managed ai_connectors"
	aiConnectorsSectionEnd   = "# END lrc managed ai_connectors"
)

type uiRuntimeConfig struct {
	APIURL        string
	JWT           string
	RefreshJWT    string
	OrgID         string
	UserEmail     string
	UserID        string
	FirstName     string
	LastName      string
	AvatarURL     string
	OrgName       string
	ConfigPath    string
	ConfigErr     string
	ConfigMissing bool
}

type aiConnectorRemote struct {
	ID            int64  `json:"id"`
	ProviderName  string `json:"provider_name"`
	ConnectorName string `json:"connector_name"`
	APIKey        string `json:"api_key"`
	BaseURL       string `json:"base_url"`
	SelectedModel string `json:"selected_model"`
	DisplayOrder  int    `json:"display_order"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type connectorManagerServer struct {
	cfg    *uiRuntimeConfig
	client *http.Client
	mu     sync.Mutex
}

type authRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func runUI(c *cli.Context) error {
	cfg, err := loadUIRuntimeConfig()
	if err != nil {
		return err
	}

	ln, port, err := pickServePort(defaultUIPort, 20)
	if err != nil {
		return fmt.Errorf("failed to reserve UI port: %w", err)
	}

	srv := &connectorManagerServer{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", getStaticHandler()))
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/api/ui/session-status", srv.handleSessionStatus)
	mux.HandleFunc("/api/ui/auth/reauth", srv.handleReauthenticate)
	mux.HandleFunc("/api/ui/connectors/reorder", srv.handleReorder)
	mux.HandleFunc("/api/ui/connectors/validate-key", srv.handleValidateKey)
	mux.HandleFunc("/api/ui/connectors/ollama/models", srv.handleOllamaModels)
	mux.HandleFunc("/api/ui/connectors/", srv.handleConnectorByID)
	mux.HandleFunc("/api/ui/connectors", srv.handleConnectors)

	httpServer := &http.Server{Handler: mux}
	go func() {
		if err := httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("ui server error: %v", err)
		}
	}()

	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("\n🌐 git-lrc Manager UI available at: %s\n\n", highlightURL(url))
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = openURL(url)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(ctx)
}

func loadUIRuntimeConfig() (*uiRuntimeConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".lrc.toml")
	cfg := &uiRuntimeConfig{
		APIURL:        defaultAPIURL,
		ConfigPath:    configPath,
		ConfigMissing: false,
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			cfg.ConfigErr = fmt.Sprintf("config file not found at %s", configPath)
			cfg.ConfigMissing = true
			return cfg, nil
		}
		cfg.ConfigErr = fmt.Sprintf("failed to read config file %s: %v", configPath, err)
		return cfg, nil
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(configPath), toml.Parser()); err != nil {
		cfg.ConfigErr = fmt.Sprintf("failed to load config file %s: %v", configPath, err)
		return cfg, nil
	}

	apiURL := strings.TrimSpace(k.String("api_url"))
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	cfg.APIURL = apiURL
	cfg.JWT = strings.TrimSpace(k.String("jwt"))
	cfg.RefreshJWT = strings.TrimSpace(k.String("refresh_token"))
	cfg.OrgID = strings.TrimSpace(k.String("org_id"))
	cfg.UserEmail = strings.TrimSpace(k.String("user_email"))
	cfg.UserID = strings.TrimSpace(k.String("user_id"))
	cfg.FirstName = strings.TrimSpace(k.String("user_first_name"))
	cfg.LastName = strings.TrimSpace(k.String("user_last_name"))
	cfg.AvatarURL = strings.TrimSpace(k.String("avatar_url"))
	cfg.OrgName = strings.TrimSpace(k.String("org_name"))

	return cfg, nil
}

type uiSessionStatusResponse struct {
	Authenticated  bool   `json:"authenticated"`
	SessionExpired bool   `json:"session_expired"`
	MissingConfig  bool   `json:"missing_config"`
	DisplayName    string `json:"display_name,omitempty"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	UserEmail      string `json:"user_email,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	OrgID          string `json:"org_id,omitempty"`
	OrgName        string `json:"org_name,omitempty"`
	APIURL         string `json:"api_url"`
	Message        string `json:"message,omitempty"`
}

func (s *connectorManagerServer) handleSessionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.Lock()
	jwt := strings.TrimSpace(s.cfg.JWT)
	orgID := strings.TrimSpace(s.cfg.OrgID)
	apiURL := strings.TrimSpace(s.cfg.APIURL)
	userEmail := strings.TrimSpace(s.cfg.UserEmail)
	userID := strings.TrimSpace(s.cfg.UserID)
	firstName := strings.TrimSpace(s.cfg.FirstName)
	lastName := strings.TrimSpace(s.cfg.LastName)
	avatarURL := strings.TrimSpace(s.cfg.AvatarURL)
	orgName := strings.TrimSpace(s.cfg.OrgName)
	configErr := strings.TrimSpace(s.cfg.ConfigErr)
	configMissing := s.cfg.ConfigMissing
	s.mu.Unlock()

	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	claims := decodeJWTClaims(jwt)
	if userEmail == "" {
		userEmail = strings.TrimSpace(claims["email"])
	}
	if firstName == "" {
		firstName = firstNonEmpty(claims["given_name"], claims["first_name"])
	}
	if lastName == "" {
		lastName = firstNonEmpty(claims["family_name"], claims["last_name"])
	}
	if avatarURL == "" {
		avatarURL = firstNonEmpty(claims["picture"], claims["avatar_url"])
	}
	displayName := strings.TrimSpace(claims["name"])
	if displayName == "" {
		displayName = strings.TrimSpace(strings.TrimSpace(firstName + " " + lastName))
	}
	if displayName == "" {
		displayName = firstNonEmpty(userEmail, userID)
	}

	status := uiSessionStatusResponse{
		Authenticated:  false,
		SessionExpired: false,
		MissingConfig:  configMissing,
		DisplayName:    displayName,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarURL:      avatarURL,
		UserEmail:      userEmail,
		UserID:         userID,
		OrgID:          orgID,
		OrgName:        orgName,
		APIURL:         apiURL,
	}

	if jwt == "" || orgID == "" {
		if configErr != "" {
			status.Message = configErr
		} else {
			status.Message = "not authenticated"
		}
		writeJSON(w, http.StatusOK, status)
		return
	}

	probeURL := buildLiveReviewURL(apiURL, "/api/v1/aiconnectors")
	probeStatus, _, err := s.forwardJSONRequest(http.MethodGet, probeURL, nil, jwt, orgID)
	if err != nil {
		status.Message = err.Error()
		writeJSON(w, http.StatusOK, status)
		return
	}

	if probeStatus == http.StatusUnauthorized {
		refreshed, refreshErr := s.refreshAccessToken(jwt)
		if refreshErr != nil || !refreshed {
			status.SessionExpired = true
			if refreshErr != nil {
				status.Message = refreshErr.Error()
			} else {
				status.Message = "session expired"
			}
			writeJSON(w, http.StatusOK, status)
			return
		}

		s.mu.Lock()
		jwt = strings.TrimSpace(s.cfg.JWT)
		s.mu.Unlock()
		probeStatus, _, err = s.forwardJSONRequest(http.MethodGet, probeURL, nil, jwt, orgID)
		if err != nil {
			status.Message = err.Error()
			writeJSON(w, http.StatusOK, status)
			return
		}
	}

	if probeStatus >= 200 && probeStatus < 300 {
		status.Authenticated = true
		status.SessionExpired = false
		status.Message = "authenticated"
		writeJSON(w, http.StatusOK, status)
		return
	}

	status.Message = fmt.Sprintf("session check failed with status %d", probeStatus)
	writeJSON(w, http.StatusOK, status)
}

func (s *connectorManagerServer) handleReauthenticate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slog := newSetupLog()
	result, err := runHexmosLoginFlow(slog)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("reauthentication failed: %v", err))
		return
	}

	if err := writeConfig(result); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to persist session: %v", err))
		return
	}

	_ = os.Remove(slog.logFile)

	s.mu.Lock()
	s.cfg.APIURL = cloudAPIURL
	s.cfg.JWT = strings.TrimSpace(result.AccessToken)
	s.cfg.RefreshJWT = strings.TrimSpace(result.RefreshToken)
	s.cfg.OrgID = strings.TrimSpace(result.OrgID)
	s.cfg.UserEmail = strings.TrimSpace(result.Email)
	s.cfg.UserID = strings.TrimSpace(result.UserID)
	s.cfg.FirstName = strings.TrimSpace(result.FirstName)
	s.cfg.LastName = strings.TrimSpace(result.LastName)
	s.cfg.AvatarURL = strings.TrimSpace(result.AvatarURL)
	s.cfg.OrgName = strings.TrimSpace(result.OrgName)
	s.cfg.ConfigErr = ""
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, uiSessionStatusResponse{
		Authenticated:  true,
		SessionExpired: false,
		MissingConfig:  false,
		DisplayName:    strings.TrimSpace(strings.TrimSpace(result.FirstName + " " + result.LastName)),
		FirstName:      strings.TrimSpace(result.FirstName),
		LastName:       strings.TrimSpace(result.LastName),
		AvatarURL:      strings.TrimSpace(result.AvatarURL),
		UserEmail:      strings.TrimSpace(result.Email),
		UserID:         strings.TrimSpace(result.UserID),
		OrgID:          strings.TrimSpace(result.OrgID),
		OrgName:        strings.TrimSpace(result.OrgName),
		APIURL:         cloudAPIURL,
		Message:        "reauthentication complete",
	})
}

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

func (s *connectorManagerServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	htmlBytes, err := staticFiles.ReadFile("static/ui-connectors.html")
	if err != nil {
		http.Error(w, "failed to load UI", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.Copy(w, bytes.NewReader(htmlBytes)); err != nil {
		log.Printf("failed to write UI index response: %v", err)
	}
}

func (s *connectorManagerServer) handleConnectors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status, body, err := s.proxyJSONRequest(http.MethodGet, "/api/v1/aiconnectors", nil)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}

		if status >= 200 && status < 300 {
			var connectors []aiConnectorRemote
			if err := json.Unmarshal(body, &connectors); err != nil {
				log.Printf("failed to decode connectors response for config persistence: %v", err)
			} else {
				if err := persistConnectorsToConfig(s.cfg.ConfigPath, connectors); err != nil {
					log.Printf("failed to persist connectors to config: %v", err)
				}
			}
		}

		writeRawJSON(w, status, body)
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		status, respBody, err := s.proxyJSONRequest(http.MethodPost, "/api/v1/aiconnectors", body)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeRawJSON(w, status, respBody)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *connectorManagerServer) handleConnectorByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/ui/connectors/")
	if id == "" || strings.Contains(id, "/") {
		writeJSONError(w, http.StatusNotFound, "connector not found")
		return
	}

	apiPath := "/api/v1/aiconnectors/" + id

	switch r.Method {
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		status, respBody, err := s.proxyJSONRequest(http.MethodPut, apiPath, body)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeRawJSON(w, status, respBody)
	case http.MethodDelete:
		status, respBody, err := s.proxyJSONRequest(http.MethodDelete, apiPath, nil)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err.Error())
			return
		}
		writeRawJSON(w, status, respBody)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *connectorManagerServer) handleReorder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	status, respBody, err := s.proxyJSONRequest(http.MethodPut, "/api/v1/aiconnectors/reorder", body)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeRawJSON(w, status, respBody)
}

func (s *connectorManagerServer) handleValidateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	status, respBody, err := s.proxyJSONRequest(http.MethodPost, "/api/v1/aiconnectors/validate-key", body)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeRawJSON(w, status, respBody)
}

func (s *connectorManagerServer) handleOllamaModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	status, respBody, err := s.proxyJSONRequest(http.MethodPost, "/api/v1/aiconnectors/ollama/models", body)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeRawJSON(w, status, respBody)
}

func (s *connectorManagerServer) proxyJSONRequest(method, apiPath string, payload []byte) (int, []byte, error) {
	url := buildLiveReviewURL(s.cfg.APIURL, apiPath)

	s.mu.Lock()
	jwt := s.cfg.JWT
	orgID := s.cfg.OrgID
	s.mu.Unlock()

	if strings.TrimSpace(jwt) == "" || strings.TrimSpace(orgID) == "" {
		return http.StatusUnauthorized, []byte(`{"error":"not authenticated. Open Home and use Re-authenticate."}`), nil
	}

	status, respBody, err := s.forwardJSONRequest(method, url, payload, jwt, orgID)
	if err != nil {
		return status, nil, err
	}

	if status == http.StatusUnauthorized {
		refreshed, refreshErr := s.refreshAccessToken(jwt)
		if refreshErr != nil {
			log.Printf("failed to refresh lrc ui token: %v", refreshErr)
			return status, respBody, nil
		}
		if refreshed {
			s.mu.Lock()
			newJWT := s.cfg.JWT
			s.mu.Unlock()

			status, retryBody, retryErr := s.forwardJSONRequest(method, url, payload, newJWT, orgID)
			if retryErr != nil {
				return status, nil, retryErr
			}
			return status, retryBody, nil
		}
	}

	return status, respBody, nil
}

func (s *connectorManagerServer) forwardJSONRequest(method, url string, payload []byte, jwt string, orgID string) (int, []byte, error) {

	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("X-Org-Context", orgID)

	resp, err := s.client.Do(req)
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("failed to call LiveReview API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("failed to read LiveReview API response: %w", err)
	}

	return resp.StatusCode, respBody, nil
}

func (s *connectorManagerServer) refreshAccessToken(failedJWT string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.cfg.JWT) != strings.TrimSpace(failedJWT) {
		return true, nil
	}

	if strings.TrimSpace(s.cfg.RefreshJWT) == "" {
		return false, fmt.Errorf("refresh_token missing in %s", s.cfg.ConfigPath)
	}

	refreshURL := buildLiveReviewURL(s.cfg.APIURL, "/api/v1/auth/refresh")
	reqBody, err := json.Marshal(authRefreshRequest{RefreshToken: s.cfg.RefreshJWT})
	if err != nil {
		return false, fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, refreshURL, bytes.NewReader(reqBody))
	if err != nil {
		return false, fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read refresh response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("refresh failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var refreshResp authRefreshResponse
	if err := json.Unmarshal(body, &refreshResp); err != nil {
		return false, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	if strings.TrimSpace(refreshResp.AccessToken) == "" {
		return false, fmt.Errorf("refresh response missing access token")
	}

	s.cfg.JWT = strings.TrimSpace(refreshResp.AccessToken)
	if strings.TrimSpace(refreshResp.RefreshToken) != "" {
		s.cfg.RefreshJWT = strings.TrimSpace(refreshResp.RefreshToken)
	}

	if err := persistAuthTokensToConfig(s.cfg.ConfigPath, s.cfg.JWT, s.cfg.RefreshJWT); err != nil {
		log.Printf("warning: refreshed token obtained but failed to update %s: %v", s.cfg.ConfigPath, err)
	}

	return true, nil
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

func persistConnectorsToConfig(configPath string, connectors []aiConnectorRemote) error {
	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config for connector snapshot: %w", err)
		}
		originalBytes = []byte{}
	}

	originalContent := string(originalBytes)
	cleanedContent := stripManagedAIConnectorsSection(originalContent)
	managedSection := renderManagedAIConnectorsSection(connectors)

	trimmed := strings.TrimRight(cleanedContent, "\n\r\t ")
	var updatedContent string
	if trimmed == "" {
		updatedContent = managedSection + "\n"
	} else {
		updatedContent = trimmed + "\n\n" + managedSection + "\n"
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(updatedContent), 0600); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func persistAuthTokensToConfig(configPath string, jwt string, refreshToken string) error {
	originalBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config for token update: %w", err)
	}

	content := string(originalBytes)
	updated := upsertQuotedConfigValue(content, "jwt", jwt)
	if strings.TrimSpace(refreshToken) != "" {
		updated = upsertQuotedConfigValue(updated, "refresh_token", refreshToken)
	}

	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func upsertQuotedConfigValue(content string, key string, value string) string {
	lines := strings.Split(content, "\n")
	prefix := key + " = "
	replacement := prefix + strconv.Quote(value)
	replaced := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = replacement
			replaced = true
			break
		}
	}

	if !replaced {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, replacement)
	}

	return strings.Join(lines, "\n")
}

func stripManagedAIConnectorsSection(content string) string {
	start := strings.Index(content, aiConnectorsSectionBegin)
	if start == -1 {
		return content
	}

	endRelative := strings.Index(content[start:], aiConnectorsSectionEnd)
	if endRelative == -1 {
		return content[:start]
	}

	end := start + endRelative + len(aiConnectorsSectionEnd)
	if end < len(content) {
		if content[end] == '\r' {
			end++
		}
		if end < len(content) && content[end] == '\n' {
			end++
		}
	}

	return content[:start] + content[end:]
}

func renderManagedAIConnectorsSection(connectors []aiConnectorRemote) string {
	var builder strings.Builder
	builder.WriteString(aiConnectorsSectionBegin)
	builder.WriteString("\n")
	builder.WriteString("# Generated by lrc ui. This section is auto-managed and will be replaced.\n")

	for _, connector := range connectors {
		builder.WriteString("\n[[ai_connectors]]\n")
		builder.WriteString("id = ")
		builder.WriteString(strconv.FormatInt(connector.ID, 10))
		builder.WriteString("\n")
		builder.WriteString("provider_name = ")
		builder.WriteString(strconv.Quote(connector.ProviderName))
		builder.WriteString("\n")
		builder.WriteString("connector_name = ")
		builder.WriteString(strconv.Quote(connector.ConnectorName))
		builder.WriteString("\n")
		builder.WriteString("api_key = ")
		builder.WriteString(strconv.Quote(connector.APIKey))
		builder.WriteString("\n")
		if connector.BaseURL != "" {
			builder.WriteString("base_url = ")
			builder.WriteString(strconv.Quote(connector.BaseURL))
			builder.WriteString("\n")
		}
		if connector.SelectedModel != "" {
			builder.WriteString("selected_model = ")
			builder.WriteString(strconv.Quote(connector.SelectedModel))
			builder.WriteString("\n")
		}
		builder.WriteString("display_order = ")
		builder.WriteString(strconv.Itoa(connector.DisplayOrder))
		builder.WriteString("\n")
		if connector.CreatedAt != "" {
			builder.WriteString("created_at = ")
			builder.WriteString(strconv.Quote(connector.CreatedAt))
			builder.WriteString("\n")
		}
		if connector.UpdatedAt != "" {
			builder.WriteString("updated_at = ")
			builder.WriteString(strconv.Quote(connector.UpdatedAt))
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n")
	builder.WriteString(aiConnectorsSectionEnd)
	return builder.String()
}
