package webscraper

import (
    "github.com/PuerkitoBio/goquery"
    "anondd/utils/models"
)

// Scraper defines the interface for scrapers
type Scraper interface {
    FetchHTML(endpoint string) (*goquery.Document, error)
    ParseAgents(doc *goquery.Document) ([]models.Agent, error)
    GetAgent(id string) (*models.Agent, error)
    StopScheduler()
}
