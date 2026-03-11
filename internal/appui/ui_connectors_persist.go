package appui

import (
	"fmt"
	"os"
	"strings"
)

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
