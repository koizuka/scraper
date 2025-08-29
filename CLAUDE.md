# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go web scraping library that provides a comprehensive toolkit for web scraping with both traditional HTTP client and Chrome/Chromium browser automation capabilities.

## Core Architecture

### Main Components

1. **Session** (`session.go`) - Central orchestrator for HTTP requests, cookie management, and file operations
   - Manages HTTP client with persistent cookies using `github.com/orirawlings/persistent-cookiejar`
   - Handles request/response logging and debugging
   - Supports offline mode by saving/loading responses from disk
   - Provides various navigation methods (GET, form submission, link following)

2. **Page** (`page.go`) - DOM representation and navigation
   - Wraps `goquery.Document` for jQuery-like DOM manipulation
   - Handles relative URL resolution and meta refresh redirects
   - Provides CSS selector-based element finding

3. **ChromeSession** (`chrome.go`) - Browser automation layer
   - Extends Session with Chrome DevTools Protocol capabilities via `github.com/chromedp/chromedp`
   - Manages Chrome user data directory and download handling
   - Provides file download with glob pattern matching
   - Supports both headless and non-headless operation

4. **Form** (`form.go`) - Form handling and submission
   - Parses HTML forms into structured data with validation
   - Supports all input types (text, select, checkbox, radio, hidden, etc.)
   - Handles form encoding and submission with proper headers

5. **Unmarshal** (`unmarshal.go`) - Data extraction and parsing
   - Provides struct-based data extraction from HTML using reflection
   - Supports custom field tags (`find`, `attr`, `re`, `time`, `html`, `ignore`)
   - Handles nested structures and slices
   - Includes number extraction utilities with regex parsing

6. **Response** (`response.go`) - HTTP response processing
   - Handles character encoding detection and conversion
   - Provides DOM parsing and filtering capabilities
   - Supports custom body filtering functions

## Key Features

- **Dual Mode Operation**: Traditional HTTP client + Chrome browser automation
- **Persistent Cookies**: Automatic cookie persistence across sessions
- **Offline Development**: Save/replay HTTP responses for development
- **Form Automation**: Comprehensive form parsing and submission
- **Data Extraction**: Struct-based HTML parsing with flexible field mapping
- **File Downloads**: Managed downloads with pattern matching
- **Encoding Support**: Automatic charset detection and conversion
- **Debugging**: Comprehensive logging of requests, responses, and form data

## Common Development Commands

```bash
# Run tests
go test ./...

# Run specific test
go test -run TestFunctionName

# Build
go build

# Install dependencies
go mod tidy

# Run tests with verbose output
go test -v ./...
```

## Testing

Tests are located in `*_test.go` files. Key test files:

- `chrome_test.go` - Chrome automation tests
- `form_test.go` - Form handling tests
- `response_test.go` - Response processing tests
- `unmarshal_test.go` - Data extraction tests

## Usage Patterns

### Basic Scraping Session

```go
var logger scraper.ConsoleLogger
session := scraper.NewSession("session-name", logger)
err := session.LoadCookie()
page, err := session.GetPage("https://example.com")
```

### Chrome Automation

```go
chromeSession, cancel, err := session.NewChrome()
defer cancel()
resp, err := chromeSession.RunNavigate("https://example.com")
```

### Data Extraction

```go
type Item struct {
    Title string `find:"h2"`
    Link  string `find:"a" attr:"href"`
    Price float64 `find:".price" re:"([0-9.]+)"`
}
var items []Item
err := scraper.Unmarshal(&items, page.Find(".item"), scraper.UnmarshalOption{})
```

## Dependencies

- `github.com/PuerkitoBio/goquery` - HTML parsing and manipulation
- `github.com/chromedp/chromedp` - Chrome DevTools Protocol client
- `github.com/orirawlings/persistent-cookiejar` - Persistent cookie storage
- `golang.org/x/text` - Text encoding conversion

## Development Guidelines

- git commit する前には go fmt を実行してください
