package setup

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// WaitForLoginCallback waits for login callback data, server error, or timeout.
// It ensures temporary server shutdown on completion paths.
func WaitForLoginCallback(dataCh <-chan *HexmosCallbackData, errCh <-chan error, server *http.Server, timeout time.Duration) (*HexmosCallbackData, error) {
	select {
	case cbData := <-dataCh:
		go server.Shutdown(context.Background())
		return cbData, nil
	case err := <-errCh:
		server.Shutdown(context.Background())
		return nil, err
	case <-time.After(timeout):
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("timed out waiting for login (5 minutes)")
	}
}
