package main

import (
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", log.Lshortfile|log.LstdFlags)

func logf(f string, v ...interface{}) {
	if config.Verbose {
		logger.Output(2, fmt.Sprintf(f, v...))
	}
}

type logHelper struct {
	prefix string
}

func (l *logHelper) Write(p []byte) (n int, err error) {
	if config.Verbose {
		logger.Printf("%s%s\n", l.prefix, p)
		return len(p), nil
	}
	return
}

func newLogHelper(prefix string) *logHelper {
	return &logHelper{prefix}
}
