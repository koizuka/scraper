package scraper

import "fmt"

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
}

func (error UnexpectedContentTypeError) Error() string {
	return "Unexpected Content-Type received"
}

type MaintenanceError struct {
	Message string
}

func (error MaintenanceError) Error() string {
	return fmt.Sprintf("Service is under maintenance. %v", error.Message)
}
