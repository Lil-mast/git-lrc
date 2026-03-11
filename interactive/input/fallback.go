package input

import (
	"bufio"
	"errors"
	"os"
	"runtime"
)

var ErrInputCancelled = errors.New("terminal input cancelled")

const DecisionCommit = 0

// OpenTTY opens the controlling terminal for reading.
// On Unix this is /dev/tty; on Windows it is CONIN$.
func OpenTTY() (*os.File, error) {
	if runtime.GOOS == "windows" {
		return os.OpenFile("CONIN$", os.O_RDWR, 0)
	}
	return os.Open("/dev/tty")
}

// HandleEnterFallbackWithCancel waits for a newline in cooked mode and maps it
// to a commit decision. This is a fallback for terminals where raw key capture
// cannot attach reliably.
func HandleEnterFallbackWithCancel(stop <-chan struct{}) (int, error) {
	tty, err := OpenTTY()
	if err != nil {
		return 0, err
	}

	reader := bufio.NewReader(tty)
	lineCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		_, readErr := reader.ReadString('\n')
		if readErr != nil {
			errCh <- readErr
			return
		}
		lineCh <- struct{}{}
	}()

	select {
	case <-stop:
		_ = tty.Close()
		<-done
		return 0, ErrInputCancelled
	case <-lineCh:
		_ = tty.Close()
		<-done
		return DecisionCommit, nil
	case readErr := <-errCh:
		_ = tty.Close()
		<-done
		return 0, readErr
	}
}
