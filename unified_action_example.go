package scraper

import (
	"fmt"
	"time"
)

// Example showing how the new action-based UnifiedScraper would work
// This is a demonstration of migrating JCB Card login code to the new API

func ExampleJCBCardLoginWithActions(scraper UnifiedScraper, userId, password string) error {
	// This is how JCB Card login would look with the new action-based API
	// Compare this to the original chromedp.Run() style in jcbCard.go

	err := scraper.Run(
		// Navigation and initial setup
		Navigate("https://my.jcb.co.jp/Login"),
		WaitVisible("form[name='loginForm']"),
		SavePage(),

		// Login form filling
		SendKeys("#userId", userId),
		SendKeys("#password", password),
		Sleep(2*time.Second), // Wait for validation

		// Submit login
		Click("#loginButtonAD"),
		Sleep(1*time.Second), // Wait for page transition
		SavePage(),
	)
	if err != nil {
		return fmt.Errorf("JCB login failed: %w", err)
	}

	// Check current URL after login (this would need conditional logic)
	currentURL, err := scraper.GetCurrentURL()
	if err != nil {
		return err
	}
	scraper.Printf("Current URL after login: %s", currentURL)

	return nil
}

// Example of conditional actions - this shows how complex logic would work
func ExampleConditionalActions(scraper UnifiedScraper) error {
	// First, navigate and get the current state
	err := scraper.Run(
		Navigate("https://example.com/login"),
		SavePage(),
	)
	if err != nil {
		return err
	}

	// Check current URL to decide next action
	currentURL, err := scraper.GetCurrentURL()
	if err != nil {
		return err
	}

	// Based on URL, perform different actions
	if currentURL == "https://login.yahoo.co.jp" {
		// Yahoo login detected
		err = scraper.Run(
			WaitVisible(`input[name="handle"]`),
			SendKeys(`input[name="handle"]`, "user@example.com"),
			Sleep(1*time.Second),
			Click(`button[class*="riff-bg-key"]`),
			Sleep(2*time.Second),
			SavePage(),
		)
	} else {
		// Regular login
		err = scraper.Run(
			WaitVisible("#loginForm"),
			SendKeys("#username", "user@example.com"),
			SendKeys("#password", "password123"),
			Click("#submit"),
			SavePage(),
		)
	}

	return err
}

// Example of data extraction with actions
func ExampleDataExtractionWithActions(scraper UnifiedScraper) error {
	// Navigate to a page and extract data
	err := scraper.Run(
		Navigate("https://example.com/data"),
		WaitVisible(".data-table"),
		SavePage(),
	)
	if err != nil {
		return err
	}

	// Extract data using the new action
	var data struct {
		Items []struct {
			Name  string `find:".item-name"`
			Price string `find:".item-price"`
		} `find:".item-row"`
	}

	err = scraper.Run(
		ExtractData(&data, ".data-table", UnmarshalOption{}),
	)
	if err != nil {
		return err
	}

	scraper.Printf("Extracted %d items", len(data.Items))
	return nil
}

// Example showing how to create custom actions
func CustomLoginAction(site, username, password string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		switch site {
		case "jcb":
			return scraper.Run(
				Navigate("https://my.jcb.co.jp/Login"),
				WaitVisible("form[name='loginForm']"),
				SendKeys("#userId", username),
				SendKeys("#password", password),
				Click("#loginButtonAD"),
			)
		case "yahoo":
			return scraper.Run(
				Navigate("https://login.yahoo.co.jp"),
				WaitVisible(`input[name="handle"]`),
				SendKeys(`input[name="handle"]`, username),
				Click(`button[class*="riff-bg-key"]`),
			)
		default:
			return fmt.Errorf("unsupported site: %s", site)
		}
	})
}

// Usage of custom action
func ExampleCustomAction(scraper UnifiedScraper) error {
	return scraper.Run(
		CustomLoginAction("jcb", "myuser", "mypass"),
		SavePage(),
		// Continue with other actions...
	)
}