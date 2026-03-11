package appui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

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
