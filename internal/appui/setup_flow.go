package appui

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	setuptpl "github.com/HexmosTech/git-lrc/setup"
	"github.com/urfave/cli/v2"
)

// RunSetup is the handler for "lrc setup".
func RunSetup(c *cli.Context) error {
	slog := newSetupLog()

	fmt.Println()
	fmt.Printf("  %s%s🔧 git-lrc setup%s\n", clr(cBold), clr(cCyan), clr(cReset))
	fmt.Printf("  %s───────────────────%s\n", clr(cDim), clr(cReset))
	fmt.Println()

	if err := backupExistingConfig(slog); err != nil {
		return setupError(slog, err)
	}

	fmt.Printf("  %s%sStep 1/2%s  🔑 Authenticate with Hexmos\n", clr(cBold), clr(cBlue), clr(cReset))
	fmt.Println()
	slog.write("phase 1: starting hexmos login flow")

	result, err := runHexmosLoginFlow(slog)
	if err != nil {
		return setupError(slog, fmt.Errorf("authentication failed: %w", err))
	}

	fmt.Printf("  %s✅ Authenticated as %s%s%s\n", clr(cGreen), clr(cBold), result.Email, clr(cReset))
	if result.OrgName != "" {
		fmt.Printf("  %s   Organization: %s%s\n", clr(cDim), result.OrgName, clr(cReset))
	}
	fmt.Println()
	slog.write("phase 1 complete: user=%s org=%s", result.Email, result.OrgID)

	fmt.Printf("  %s%sStep 2/2%s  🤖 Configure AI (Gemini)\n", clr(cBold), clr(cBlue), clr(cReset))
	fmt.Println()
	fmt.Printf("  You need a Gemini API key for AI-powered code reviews.\n")
	fmt.Printf("  Get a free key from: %s\n", hyperlink(geminiKeysURL, clr(cCyan)+geminiKeysURL+clr(cReset)))
	fmt.Println()
	slog.write("phase 2: prompting for gemini key")

	if err := openURL(geminiKeysURL); err != nil {
		slog.write("warning: failed to auto-open Gemini keys URL: %v", err)
		fmt.Printf("  %s⚠ Could not open browser automatically.%s Open this URL manually: %s\n", clr(cYellow), clr(cReset), hyperlink(geminiKeysURL, clr(cCyan)+geminiKeysURL+clr(cReset)))
		fmt.Println()
	}

	geminiKey, err := promptGeminiKey(result, slog)
	if err != nil {
		return setupError(slog, fmt.Errorf("gemini setup failed: %w", err))
	}

	slog.write("creating gemini connector")
	if err := createGeminiConnector(result, geminiKey); err != nil {
		return setupError(slog, fmt.Errorf("failed to create AI connector: %w", err))
	}
	fmt.Printf("  %s✅ Gemini connector created%s %s(model: %s)%s\n", clr(cGreen), clr(cReset), clr(cDim), defaultGeminiModel, clr(cReset))
	fmt.Println()
	slog.write("gemini connector created")

	if err := writeConfig(result); err != nil {
		return setupError(slog, fmt.Errorf("failed to write config: %w", err))
	}
	slog.write("config written to ~/.lrc.toml")

	printSetupSuccess(result)

	if err := os.Remove(slog.logFile); err != nil && !os.IsNotExist(err) {
		slog.write("warning: could not remove log file: %v", err)
	}
	return nil
}

// backupExistingConfig backs up ~/.lrc.toml if it exists and contains an api_key.
func backupExistingConfig(slog *setupLog) error {
	backupPath, err := setuptpl.BackupExistingConfig(slog.write)
	if err != nil {
		return err
	}
	if backupPath != "" {
		fmt.Printf("  %s📦 Existing config backed up to:%s %s%s%s\n", clr(cYellow), clr(cReset), clr(cDim), backupPath, clr(cReset))
		fmt.Println()
	}
	return nil
}

// runHexmosLoginFlow starts a temporary server, opens the browser for Hexmos Login,
// waits for the callback, and provisions the user in LiveReview.
func runHexmosLoginFlow(slog *setupLog) (*setupResult, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	dataCh := make(chan *hexmosCallbackData, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()

	signinURL := setuptpl.BuildSigninURL(callbackURL)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := setupLandingPageTemplate.Execute(w, struct{ SigninURL string }{SigninURL: signinURL}); err != nil {
			http.Error(w, "failed to render setup page", http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		dataParam := r.URL.Query().Get("data")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		cbData, err := setuptpl.ProcessLoginCallback(
			dataParam,
			func() error { return setupErrorPageTemplate.Execute(w, nil) },
			func() error { return setupSuccessPageTemplate.Execute(w, nil) },
			slog.write,
		)
		if err != nil {
			errCh <- err
			return
		}
		dataCh <- cbData
	})

	server := setuptpl.StartTemporaryServer(listener, mux, errCh)

	localURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	fmt.Printf("  🌐 Opening browser for Hexmos Login...\n")
	fmt.Printf("     %sIf it doesn't open, visit:%s %s\n", clr(cDim), clr(cReset), hyperlink(localURL, clr(cCyan)+localURL+clr(cReset)))
	fmt.Println()
	slog.write("local server on port %d, signin url: %s", port, signinURL)

	if err := openURL(localURL); err != nil {
		slog.write("warning: failed to auto-open local login URL: %v", err)
		fmt.Printf("  %s⚠ Could not open browser automatically.%s Continue by opening: %s\n", clr(cYellow), clr(cReset), hyperlink(localURL, clr(cCyan)+localURL+clr(cReset)))
		fmt.Println()
	}

	cbData, err := setuptpl.WaitForLoginCallback(dataCh, errCh, server, setupTimeout)
	if err != nil {
		return nil, err
	}

	slog.write("callback received, provisioning user")

	return provisionLiveReviewUser(cbData, slog)
}

// provisionLiveReviewUser calls ensure-cloud-user and creates an API key.
func provisionLiveReviewUser(cbData *hexmosCallbackData, slog *setupLog) (*setupResult, error) {
	return setuptpl.ProvisionLiveReviewUser(cbData, slog.write)
}

// promptGeminiKey reads the Gemini API key from stdin with up to 3 attempts.
func promptGeminiKey(result *setupResult, slog *setupLog) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("  %s🔑 Paste your Gemini API key:%s ", clr(cBold), clr(cReset))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		key := strings.TrimSpace(line)
		if key == "" {
			fmt.Printf("  %s⚠  Key cannot be empty. Please try again.%s\n", clr(cYellow), clr(cReset))
			continue
		}

		slog.write("validating gemini key (attempt %d)", attempt)

		valid, msg, err := validateGeminiKey(result, key)
		if err != nil {
			slog.write("gemini key validation error: %v", err)
			fmt.Printf("  %s❌ Validation error: %v%s\n", clr(cRed), err, clr(cReset))
			if attempt < 3 {
				fmt.Printf("  %sPlease try again.%s\n", clr(cDim), clr(cReset))
			}
			continue
		}

		if !valid {
			slog.write("gemini key invalid: %s", msg)
			fmt.Printf("  %s❌ Invalid key: %s%s\n", clr(cRed), msg, clr(cReset))
			if attempt < 3 {
				fmt.Printf("  %sPlease try again.%s\n", clr(cDim), clr(cReset))
			}
			continue
		}

		slog.write("gemini key validated successfully")
		fmt.Printf("  %s✅ Key validated%s\n", clr(cGreen), clr(cReset))
		return key, nil
	}

	return "", fmt.Errorf("failed to provide a valid Gemini API key after 3 attempts")
}

func validateGeminiKey(result *setupResult, geminiKey string) (bool, string, error) {
	return setuptpl.ValidateGeminiKey(result, geminiKey)
}

func createGeminiConnector(result *setupResult, geminiKey string) error {
	return setuptpl.CreateGeminiConnector(result, geminiKey)
}

func writeConfig(result *setupResult) error {
	return setuptpl.WriteConfig(result)
}

func writeFileAtomically(path string, data []byte, mode os.FileMode) error {
	return setuptpl.WriteFileAtomically(path, data, mode)
}
