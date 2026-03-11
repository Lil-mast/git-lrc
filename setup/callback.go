package setup

import (
	"encoding/json"
	"fmt"
)

// ParseAndValidateCallbackData parses and validates callback payload from Hexmos login.
func ParseAndValidateCallbackData(dataParam string) (*HexmosCallbackData, error) {
	if dataParam == "" {
		return nil, fmt.Errorf("no data parameter in callback")
	}

	var cbData HexmosCallbackData
	if err := json.Unmarshal([]byte(dataParam), &cbData); err != nil {
		return nil, fmt.Errorf("failed to parse callback data: %w", err)
	}

	if cbData.Result.JWT == "" || cbData.Result.Data.Email == "" {
		return nil, fmt.Errorf("incomplete callback data (missing JWT or email)")
	}

	return &cbData, nil
}

// ProcessLoginCallback validates callback data and invokes the appropriate render hook.
// On validation failure it renders error page and returns an error.
// On success it renders success page and returns callback data.
func ProcessLoginCallback(dataParam string, renderError func() error, renderSuccess func() error, logf func(format string, args ...interface{})) (*HexmosCallbackData, error) {
	cbData, err := ParseAndValidateCallbackData(dataParam)
	if err != nil {
		if renderError != nil {
			if writeErr := renderError(); writeErr != nil && logf != nil {
				logf("warning: failed to write setup error page: %v", writeErr)
			}
		}
		return nil, err
	}

	if renderSuccess != nil {
		if err := renderSuccess(); err != nil {
			return nil, fmt.Errorf("failed to write setup success page: %w", err)
		}
	}

	return cbData, nil
}
