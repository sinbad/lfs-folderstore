package util

import (
	"bufio"
	"strings"
)

// WriteToStderr is for when you need to print extra information
func WriteToStderr(msg string, errWriter *bufio.Writer) {
	if !strings.HasSuffix(msg, "\n") {
		msg = msg + "\n"
	}
	errWriter.WriteString(msg)
	errWriter.Flush()
}
