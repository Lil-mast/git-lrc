package appcore

import (
	"os"

	"github.com/HexmosTech/git-lrc/interactive"
)

func syncedPrintf(format string, args ...interface{}) {
	interactive.SyncedPrintf(format, args...)
}

func syncedPrintln(args ...interface{}) {
	interactive.SyncedPrintln(args...)
}

func syncedFprintf(file *os.File, format string, args ...interface{}) {
	interactive.SyncedFprintf(file, format, args...)
}

func syncFileSafely(file *os.File) {
	interactive.SyncFileSafely(file)
}
