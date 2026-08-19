package scraper

import (
	"bytes"
	"fmt"
)

type Logger interface {
	Printf(format string, a ...interface{})
}

type BufferedLogger struct {
	buffer bytes.Buffer
}

func (buflog *BufferedLogger) Printf(format string, a ...interface{}) {
	fmt.Fprintf(&buflog.buffer, format, a...)
}

// String returns the buffered log contents without consuming them.
func (buflog *BufferedLogger) String() string {
	return buflog.buffer.String()
}

func (buflog *BufferedLogger) Flush(logger Logger) {
	s := buflog.buffer.String()
	if s != "" {
		logger.Printf("%v", s)
	}
}
