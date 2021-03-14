package scraper

import "fmt"

type ConsoleLogger struct{}

func (logger ConsoleLogger) Printf(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	fmt.Println()
}
