package webscraper

import (
    "fmt"
    "log"
    "net/http"
    "github.com/PuerkitoBio/goquery"
)

// Scraper defines the structure for the scraping utility.
type Scraper struct {
    BaseURL string
    Logger  *log.Logger
}

// NewScraper initializes a new scraper.
func NewScraper(baseURL string, logger *log.Logger) *Scraper {
    return &Scraper{
        BaseURL: baseURL,
        Logger:  logger,
    }
}

// FetchHTML fetches the raw HTML from a given endpoint.
func (s *Scraper) FetchHTML(endpoint string) (*goquery.Document, error) {
    url := s.BaseURL + endpoint
    s.Logger.Printf("Fetching URL: %s", url)

    resp, err := http.Get(url)
    if (err != nil) {
        return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code %d for URL %s", resp.StatusCode, url)
    }

    doc, err := goquery.NewDocumentFromReader(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to parse HTML for URL %s: %w", url, err)
    }

    return doc, nil
}
