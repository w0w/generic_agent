package webscraper

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/robfig/cron/v3"

	"anondd/utils/models"
	"anondd/utils/storage"
)

// AgentData represents the structure of an agent's information
type AgentData struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Stats       string    `json:"stats"`
	Price       string    `json:"price"`
	ScrapedAt   time.Time `json:"scraped_at"`
}

// ScrapeJob handles scheduled scraping
type ScrapeJob struct {
	scraper *VirtualsScraper
	cron    *cron.Cron
	logger  *log.Logger
}

// VirtualsScraper implements the scraper for app.virtuals.io
type VirtualsScraper struct {
	baseURL   string
	logger    *log.Logger
	store     *storage.AgentStore
	scheduler *ScrapeJob
	cache     struct {
		agents    []models.Agent
		lastFetch time.Time
		mu        sync.RWMutex
	}
}

// NewVirtualsScraper initializes a new scraper for app.virtuals.io
func NewVirtualsScraper(logger *log.Logger, store *storage.AgentStore) *VirtualsScraper {
    if store == nil {
        logger.Fatal("store cannot be nil")
    }
    
    vs := &VirtualsScraper{
        baseURL: "https://app.virtuals.io",
        logger:  logger,
        store:   store,
        scheduler: &ScrapeJob{
            cron:   cron.New(),
            logger: logger,
        },
    }
    
    vs.scheduler.scraper = vs
    vs.startScheduler()
    return vs
}

// ParseAgents extracts agent data from the HTML document
func (v *VirtualsScraper) ParseAgents(doc *goquery.Document) ([]models.Agent, error) {
	if doc == nil {
        return nil, fmt.Errorf("nil document provided")
    }
    
    var agents []models.Agent
    now := time.Now()

    // Debug logging
    v.logger.Printf("Starting to parse agents from document")
    
    doc.Find(".agent-card").Each(func(i int, selection *goquery.Selection) {
        agentData := models.AgentData{
            Name:        selection.Find(".agent-name").Text(),
            Description: selection.Find(".agent-description").Text(),
            Stats:       selection.Find(".agent-stats").Text(),
            Price:       selection.Find(".agent-price").Text(),
            ScrapedAt:   now,
        }
        
        agent := models.FromAgentData(&agentData)
        agents = append(agents, *agent)
    })

    v.logger.Printf("Found %d agents", len(agents))
    
    if len(agents) == 0 {
        return nil, fmt.Errorf("no agents found in document")
    }

    return agents, nil
}

// GetAgent retrieves an agent by ID
func (v *VirtualsScraper) GetAgent(id string) (*models.Agent, error) {
	return v.store.GetAgent(id)
}

func (v *VirtualsScraper) refreshCache() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		v.updateCache()
	}
}

func (v *VirtualsScraper) updateCache() {
	doc, err := v.FetchHTML("/agents")
	if err != nil {
		v.logger.Printf("Cache refresh error: %v", err)
		return
	}

	agents, err := v.ParseAgents(doc)
	if err != nil {
		v.logger.Printf("Cache parse error: %v", err)
		return
	}

	v.cache.mu.Lock()
	v.cache.agents = agents
	v.cache.lastFetch = time.Now()
	v.cache.mu.Unlock()
}

func (v *VirtualsScraper) GetAgents() []models.AgentData {
	v.cache.mu.RLock()
	defer v.cache.mu.RUnlock()

	if time.Since(v.cache.lastFetch) > time.Hour || len(v.cache.agents) == 0 {
		v.cache.mu.RUnlock()
		v.updateCache()
		v.cache.mu.RLock()
	}

	// Convert Agent to AgentData for response
	agentData := make([]models.AgentData, len(v.cache.agents))
	for i, agent := range v.cache.agents {
		agentData[i] = models.AgentData{
			Name:        agent.Name,
			Description: agent.Description,
			Stats:      agent.Stats,
			Price:      agent.Price,
			ScrapedAt:  agent.ScrapedAt,
		}
	}

	return agentData
}

func (v *VirtualsScraper) FindAgent(name string) *models.AgentData {
	v.cache.mu.RLock()
	agents := v.cache.agents
	v.cache.mu.RUnlock()

	var bestMatch struct {
		agent *models.Agent
		score float64
	}

	searchName := strings.ToLower(name)
	for i := range agents {
		score := calculateSimilarity(searchName, strings.ToLower(agents[i].Name))
		if score > bestMatch.score {
			bestMatch.agent = &agents[i]
			bestMatch.score = score
		}
	}

	if bestMatch.score > 0.5 && bestMatch.agent != nil {
		return &models.AgentData{
			Name:        bestMatch.agent.Name,
			Description: bestMatch.agent.Description,
			Stats:      bestMatch.agent.Stats,
			Price:      bestMatch.agent.Price,
			ScrapedAt:  bestMatch.agent.ScrapedAt,
		}
	}
	return nil
}

// Simple string similarity calculation (Levenshtein distance based)
func calculateSimilarity(a, b string) float64 {
	// Implement fuzzy matching logic here
	// For now, using simple contains check
	if strings.Contains(b, a) {
		return 1.0
	}
	return 0.0
}

func (v *VirtualsScraper) startScheduler() {
	v.scheduler.cron.AddFunc("*/1 * * * *", func() {
		v.logger.Println("Starting scheduled scrape...")
		if err := v.runScheduledScrape(); err != nil {
			v.logger.Printf("Scheduled scrape failed: %v", err)
		}
	})
	v.scheduler.cron.Start()
}

func (v *VirtualsScraper) StopScheduler() {
	if v.scheduler != nil && v.scheduler.cron != nil {
		v.scheduler.cron.Stop()
	}
}

func (v *VirtualsScraper) runScheduledScrape() error {
	doc, err := v.FetchHTML("/agents")
	if (err != nil) {
		return fmt.Errorf("fetch failed: %w", err)
	}

	_, err = v.ParseAgents(doc)
	return err
}

// FetchHTML fetches the raw HTML from a given endpoint
func (v *VirtualsScraper) FetchHTML(endpoint string) (*goquery.Document, error) {
    url := v.baseURL + endpoint
    v.logger.Printf("Fetching URL: %s", url)
    
    resp, err := http.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
    }
    defer resp.Body.Close()
    
    return goquery.NewDocumentFromReader(resp.Body)
}
