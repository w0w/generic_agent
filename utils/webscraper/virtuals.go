package webscraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// AgentData represents the structure of an agent's information
type AgentData struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Stats       string    `json:"stats"`
	Price       string    `json:"price"`
	ScrapedAt   time.Time `json:"scraped_at"`
}

// VirtualsScraper implements the scraper for app.virtuals.io
type VirtualsScraper struct {
	Scraper   // Change from *Scraper to embedded Scraper
	OutputDir string
	cache     struct {
		agents    []AgentData
		lastFetch time.Time
		mu        sync.RWMutex
	}
}

// NewVirtualsScraper initializes a new scraper for app.virtuals.io
func NewVirtualsScraper(logger *log.Logger) *VirtualsScraper {
	vs := &VirtualsScraper{
		Scraper:   *NewScraper("https://app.virtuals.io", logger), // Note the dereferencing
		OutputDir: "training_data",
	}

	// Start cache refresh goroutine
	go vs.refreshCache()
	return vs
}

// ParseData extracts agent data from the HTML document
func (v *VirtualsScraper) ParseData(doc *goquery.Document) ([]AgentData, error) {
	var agents []AgentData
	now := time.Now()

	doc.Find(".agent-card").Each(func(i int, selection *goquery.Selection) {
		agent := AgentData{
			ScrapedAt: now,
		}

		// Extract agent information using selectors
		agent.Name = selection.Find(".agent-name").Text()
		agent.Description = selection.Find(".agent-description").Text()
		agent.Stats = selection.Find(".agent-stats").Text()
		agent.Price = selection.Find(".agent-price").Text()

		agents = append(agents, agent)
	})

	if err := v.SaveToFile(agents); err != nil {
		return nil, fmt.Errorf("failed to save agents data: %w", err)
	}

	return agents, nil
}

// SaveToFile stores the scraped agent data in JSON format
func (v *VirtualsScraper) SaveToFile(agents []AgentData) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(v.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(v.OutputDir, fmt.Sprintf("agents_%s.json", timestamp))

	// Convert data to JSON
	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agents data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write agents data to file: %w", err)
	}

	v.Logger.Printf("Saved %d agents to %s", len(agents), filename)
	return nil
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
		v.Logger.Printf("Cache refresh error: %v", err)
		return
	}

	agents, err := v.ParseData(doc)
	if err != nil {
		v.Logger.Printf("Cache parse error: %v", err)
		return
	}

	v.cache.mu.Lock()
	v.cache.agents = agents
	v.cache.lastFetch = time.Now()
	v.cache.mu.Unlock()
}

func (v *VirtualsScraper) GetAgents() []AgentData {
	v.cache.mu.RLock()
	defer v.cache.mu.RUnlock()

	if time.Since(v.cache.lastFetch) > time.Hour || len(v.cache.agents) == 0 {
		v.cache.mu.RUnlock()
		v.updateCache()
		v.cache.mu.RLock()
	}

	return v.cache.agents
}

func (v *VirtualsScraper) FindAgent(name string) *AgentData {
	agents := v.GetAgents()
	bestMatch := struct {
		agent *AgentData
		score float64
	}{nil, 0}

	searchName := strings.ToLower(name)
	for i, agent := range agents {
		score := calculateSimilarity(searchName, strings.ToLower(agent.Name))
		if score > bestMatch.score {
			bestMatch.agent = &agents[i]
			bestMatch.score = score
		}
	}

	if bestMatch.score > 0.5 { // Threshold for matching
		return bestMatch.agent
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
