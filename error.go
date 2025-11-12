package scraper

import (
	"fmt"
	"time"
)

type RetryAndRecordError struct {
	Filename string
}

func (error RetryAndRecordError) Error() string {
	return fmt.Sprintf("Record file '%v' is missing while replaying! Retry with 'record' mode!", error.Filename)
}

type LoginError struct {
	Message string
}

func (error LoginError) Error() string {
	return fmt.Sprintf("Login failed. please check config. %v", error.Message)
}

type UnexpectedContentTypeError struct {
	Expected string
	Actual   string
}

func (error UnexpectedContentTypeError) Error() string {
	if error.Expected != "" || error.Actual != "" {
		return fmt.Sprintf("Unexpected Content-Type received. Expected: %v, Actual: %v", error.Expected, error.Actual)
	}
	return "Unexpected Content-Type received"
}

type MaintenanceError struct {
	Message string
}

func (error MaintenanceError) Error() string {
	return fmt.Sprintf("Service is under maintenance. %v", error.Message)
}

type ChromeTimeoutError struct {
	OriginalError error
	Duration      time.Duration
}

func (error ChromeTimeoutError) Error() string {
	return fmt.Sprintf("Chrome operation timed out after %v: %v", error.Duration, error.OriginalError)
}

func (error ChromeTimeoutError) Unwrap() error {
	return error.OriginalError
}
