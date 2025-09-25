package scraper

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleUnifiedScraping demonstrates how to use the unified Action-based scraper interface
// to write code that works with both HTTP-based and Chrome-based scraping.
func ExampleUnifiedScraping() {
	var logger ConsoleLogger
	session := NewSession("example-session", logger)

	// Example 1: Using HTTP-based scraping with unified interface
	fmt.Println("=== HTTP-based Scraping ===")
	if err := demonstrateUnifiedAPI(session); err != nil {
		log.Printf("HTTP scraping error: %v", err)
	}

	// Example 2: Using Chrome-based scraping with unified interface
	fmt.Println("\n=== Chrome-based Scraping ===")
	chromeSession, cancel, err := session.NewChrome()
	if err != nil {
		log.Printf("Chrome setup error: %v", err)
		return
	}
	defer cancel()

	if err := demonstrateUnifiedAPI(chromeSession); err != nil {
		log.Printf("Chrome scraping error: %v", err)
	}
}

// demonstrateUnifiedAPI shows how the same Action-based code works with both scraper types
func demonstrateUnifiedAPI(scraper UnifiedScraper) error {
	scraper.SetDebugStep("Demo")
	defer scraper.ClearDebugStep()

	// Use the new Action-based API - chromedp.Run() style!
	err := scraper.Run(
		Navigate("https://example.com"),
		WaitVisible("h1"), // no-op for HTTP, important for Chrome
		SavePage(),
	)
	if err != nil {
		return fmt.Errorf("navigation and page save failed: %w", err)
	}

	scraper.Printf("Page loaded and saved successfully")

	// Extract data from the page using Action API
	type PageData struct {
		Title string `find:"h1"`
		Links []struct {
			Text string `find:"a"`
			Href string `find:"a" attr:"href"`
		} `find:"a"`
	}

	var data PageData
	err = scraper.Run(
		ExtractData(&data, "body", UnmarshalOption{}),
	)
	if err != nil {
		return fmt.Errorf("data extraction failed: %w", err)
	}

	scraper.Printf("Extracted title: %s", data.Title)
	scraper.Printf("Found %d links", len(data.Links))

	return nil
}

// ExampleSwitchableScraper demonstrates how to easily switch between scraper types
// based on configuration or runtime conditions.
func ExampleSwitchableScraper(useChrome bool) {
	var logger ConsoleLogger
	session := NewSession("switchable-session", logger)

	var scraper UnifiedScraper
	var cancel context.CancelFunc

	if useChrome {
		chromeSession, c, err := session.NewChrome()
		if err != nil {
			log.Printf("Chrome setup failed: %v", err)
			return
		}
		scraper = chromeSession
		cancel = c
		defer cancel()
		log.Println("Using Chrome scraper")
	} else {
		scraper = session
		log.Println("Using HTTP scraper")
	}

	// Same code works for both scraper types
	if err := demonstrateUnifiedAPI(scraper); err != nil {
		log.Printf("Scraping failed: %v", err)
	}
}

// ExampleFormAutomation shows unified form handling with Action API
func ExampleFormAutomation(scraper UnifiedScraper) error {
	// Navigate and fill form using Action chain
	err := scraper.Run(
		Navigate("https://example.com/login"),
		WaitVisible("form[name=login]"),
		SendKeys("input[name=username]", "user@example.com"),
		SendKeys("input[name=password]", "password123"),
		Click("input[type=submit]"), // Submit by clicking submit button
		Sleep(2*time.Second),         // Wait for form submission
		WaitVisible(".dashboard"),    // Wait for successful login
	)
	if err != nil {
		return fmt.Errorf("form automation failed: %w", err)
	}

	scraper.Printf("Login successful!")
	return nil
}

// ExampleDataExtraction demonstrates advanced data extraction patterns
func ExampleDataExtraction(scraper UnifiedScraper, url string) error {
	// Navigate and extract in one chain
	type Product struct {
		Name        string   `find:".product-name"`
		Price       float64  `find:".price" re:"([0-9.]+)"`
		Description string   `find:".description" html:"true"`
		InStock     bool     `find:".stock-status"`
		Images      []string `find:".product-images img" attr:"src"`
	}

	var products []Product
	err := scraper.Run(
		Navigate(url),
		WaitVisible(".product-list"),
		ExtractData(&products, ".product-item", UnmarshalOption{}),
	)
	if err != nil {
		return fmt.Errorf("product extraction failed: %w", err)
	}

	scraper.Printf("Found %d products", len(products))
	for i, product := range products {
		scraper.Printf("Product %d: %s - $%.2f", i+1, product.Name, product.Price)
	}

	return nil
}

// ExampleConditionalActions shows how to handle conditional logic with Action API
func ExampleConditionalActions(scraper UnifiedScraper) error {
	// First navigate to the page
	err := scraper.Run(
		Navigate("https://example.com/login"),
		SavePage(),
	)
	if err != nil {
		return err
	}

	// Check current URL to decide what to do next
	currentURL, err := scraper.GetCurrentURL()
	if err != nil {
		return err
	}

	scraper.Printf("Current URL: %s", currentURL)

	// Conditional actions based on URL
	if currentURL == "https://login.example.com" {
		// Already logged in - different page
		return scraper.Run(
			WaitVisible(".dashboard"),
			SavePage(),
		)
	} else {
		// Need to login
		return scraper.Run(
			WaitVisible("form[name=login]"),
			SendKeys("input[name=username]", "user@example.com"),
			SendKeys("input[name=password]", "password123"),
			Click("input[type=submit]"),
			Sleep(2*time.Second),
			SavePage(),
		)
	}
}

// ExampleCustomAction shows how to create reusable custom actions
func LoginAction(username, password string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		return scraper.Run(
			Navigate("https://example.com/login"),
			WaitVisible("form[name=login]"),
			SendKeys("input[name=username]", username),
			SendKeys("input[name=password]", password),
			Click("input[type=submit]"),
			Sleep(2*time.Second),
		)
	})
}

// ExampleCustomActionUsage demonstrates using custom actions
func ExampleCustomActionUsage(scraper UnifiedScraper) error {
	// Use custom action in a chain
	err := scraper.Run(
		LoginAction("user@example.com", "password123"),
		WaitVisible(".dashboard"),
		SavePage(),
	)
	if err != nil {
		return fmt.Errorf("custom action failed: %w", err)
	}

	scraper.Printf("Custom action completed successfully!")
	return nil
}

// ExampleReplayModeDemo shows how the new API handles replay mode automatically
func ExampleReplayModeDemo(scraper UnifiedScraper) error {
	// Sleep will automatically be skipped in replay mode
	err := scraper.Run(
		Navigate("https://example.com"),
		Sleep(5*time.Second), // This will be fast in replay mode
		WaitVisible("h1"),
		Sleep(2*time.Second), // This too
		SavePage(),
	)
	if err != nil {
		return err
	}

	// Check if we're in replay mode
	if scraper.IsReplayMode() {
		scraper.Printf("Running in replay mode - sleeps were skipped!")
	} else {
		scraper.Printf("Running in record mode - sleeps were executed")
	}

	return nil
}