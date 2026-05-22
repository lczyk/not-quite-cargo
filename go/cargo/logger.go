package cargo

import (
	"log"
	"os"
)

// Logger is the minimal logging interface the cargo package uses.
// Implementations are responsible for prefixes, timestamps, output etc.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

// infoPrefix / warnPrefix are computed once at startup based on whether
// stderr is a TTY and whether NO_COLOR is set
// (https://no-color.org/ -- presence of the env var disables colour
// regardless of value).
var (
	infoPrefix string
	warnPrefix string
)

func init() {
	if colorEnabled() {
		infoPrefix = "\033[32m[INFO]\033[0m "
		warnPrefix = "\033[33m[WARN]\033[0m "
	} else {
		infoPrefix = ""
		warnPrefix = "warning: "
	}
}

func colorEnabled() bool {
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

type stdLogger struct{}

func (stdLogger) Infof(format string, args ...any) { log.Printf(infoPrefix+format, args...) }
func (stdLogger) Warnf(format string, args ...any) { log.Printf(warnPrefix+format, args...) }
