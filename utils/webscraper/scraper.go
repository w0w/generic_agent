package webscraper

import (
    "github.com/PuerkitoBio/goquery"
    "anondd/utils/storage"
)

// Scraper defines the interface for scrapers
type Scraper interface {
    FetchHTML(endpoint string) (*goquery.Document, error)
    ScrapeAgents() error
    GetStore() *storage.AgentStore
    StopScheduler()
}
