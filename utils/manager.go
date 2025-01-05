package utils

import (
	"log"
	"anondd/utils/storage"
	"anondd/utils/webscraper"
)

// UtilsManager handles all utility services
type UtilsManager struct {
	scraper *webscraper.VirtualsScraper
	store   *storage.AgentStore
	logger  *log.Logger
}

// NewUtilsManager creates and initializes all utilities
func NewUtilsManager(logger *log.Logger) *UtilsManager {
	store := storage.NewAgentStore("training_data", logger)
	return &UtilsManager{
		store:  store,
		logger: logger,
	}
}

// Initialize sets up the scraper and other components
func (m *UtilsManager) Initialize() error {
	m.logger.Println("Initializing VirtualsScraper...")
	// Initialize scraper with store directly
	m.scraper = webscraper.NewVirtualsScraper(m.logger, m.store)
	
	return nil
}

// GetScraper returns the configured scraper instance
func (m *UtilsManager) GetScraper() *webscraper.VirtualsScraper {
	return m.scraper
}

// GetStore returns the AgentStore instance
func (m *UtilsManager) GetStore() *storage.AgentStore {
	return m.store
}
