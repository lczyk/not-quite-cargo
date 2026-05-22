package cargo

import "log"

// Logger is the minimal logging interface the cargo package uses.
// Implementations are responsible for prefixes, timestamps, output etc.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
}

type stdLogger struct{}

func (stdLogger) Infof(format string, args ...any) { log.Printf(format, args...) }
func (stdLogger) Warnf(format string, args ...any) { log.Printf("warning: "+format, args...) }
