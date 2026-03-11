package setup

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// BuildSigninURL builds the Hexmos signin URL for setup callback flow.
func BuildSigninURL(callbackURL string) string {
	return fmt.Sprintf("%s?app=livereview&appRedirectURI=%s", HexmosSigninBase, url.QueryEscape(callbackURL))
}

// StartTemporaryServer starts an HTTP server with the given listener and handler.
// Any non-closed serve error is sent to errCh.
func StartTemporaryServer(listener net.Listener, handler http.Handler, errCh chan<- error) *http.Server {
	server := &http.Server{Handler: handler}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()
	return server
}
