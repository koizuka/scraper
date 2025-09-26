package scraper

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ExampleUnifiedScraping demonstrates how to use the unified scraper interface
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

// demonstrateUnifiedAPI shows how the same code works with both scraper types
func demonstrateUnifiedAPI(scraper UnifiedScraper) error {
	scraper.SetDebugStep("Demo")
	defer scraper.ClearDebugStep()

	// Navigate to a page
	if err := scraper.DoNavigate("https://example.com"); err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	// Wait for content to be visible (no-op for HTTP, important for Chrome)
	if err := scraper.DoWaitVisible("h1"); err != nil {
		return fmt.Errorf("wait visible failed: %w", err)
	}

	// Save the current page
	filename, err := scraper.SavePage()
	if err != nil {
		return fmt.Errorf("save page failed: %w", err)
	}
	scraper.Printf("Page saved to: %s", filename)

	// Extract data using the unified interface
	type PageData struct {
		Title string `find:"h1"`
		Links []struct {
			Text string `find:"a"`
			Href string `find:"a" attr:"href"`
		} `find:"a"`
	}

	var data PageData
	if err := scraper.ExtractData(&data, "body", UnmarshalOption{}); err != nil {
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

// ExampleFormAutomation shows unified form handling
func ExampleFormAutomation(scraper UnifiedScraper) error {
	// Navigate to login page
	if err := scraper.DoNavigate("https://example.com/login"); err != nil {
		return err
	}

	// Fill form fields
	if err := scraper.DoSendKeys("input[name=username]", "user@example.com"); err != nil {
		return err
	}

	if err := scraper.DoSendKeys("input[name=password]", "password123"); err != nil {
		return err
	}

	// Submit form
	if err := scraper.SubmitForm("form[name=login]", nil); err != nil {
		return err
	}

	// Wait for redirect/response
	if err := scraper.DoWaitVisible(".dashboard"); err != nil {
		return err
	}

	scraper.Printf("Login successful!")
	return nil
}

// ExampleDataExtraction demonstrates advanced data extraction patterns
func ExampleDataExtraction(scraper UnifiedScraper, url string) error {
	if err := scraper.DoNavigate(url); err != nil {
		return err
	}

	// Example: Extract product information
	type Product struct {
		Name        string   `find:".product-name"`
		Price       float64  `find:".price" re:"([0-9.]+)"`
		Description string   `find:".description" html:"true"`
		InStock     bool     `find:".stock-status"`
		Images      []string `find:".product-images img" attr:"src"`
	}

	var products []Product
	if err := scraper.ExtractData(&products, ".product-item", UnmarshalOption{}); err != nil {
		return fmt.Errorf("product extraction failed: %w", err)
	}

	scraper.Printf("Found %d products", len(products))
	for i, product := range products {
		scraper.Printf("Product %d: %s - $%.2f", i+1, product.Name, product.Price)
	}

	return nil
}

// ExampleFileDownload shows how to handle downloads with the unified interface
func ExampleFileDownload(scraper UnifiedScraper) error {
	if err := scraper.DoNavigate("https://example.com/download"); err != nil {
		return err
	}

	// Click download link
	if err := scraper.DoClick(".download-button"); err != nil {
		return err
	}

	// Try to download resource (implementation varies by scraper type)
	filename, err := scraper.DownloadResource(UnifiedDownloadOptions{
		Timeout: 30 * time.Second,
		Glob:    "*.pdf",
	})

	if err != nil {
		// For Chrome scraping, you might need to use the specific DownloadFile method
		scraper.Printf("Download via unified interface failed: %v", err)
		scraper.Printf("For Chrome scraping, use chromeSession.DownloadFile() for more control")
		return err
	}

	scraper.Printf("Downloaded file: %s", filename)
	return nil
}
