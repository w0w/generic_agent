package utils

import (
	"fmt"
	"log"
	"os"
	"anondd/utils/webscraper"
)

// UtilsManager handles all utility services
type UtilsManager struct {
	logger   *log.Logger
	Scraper  *webscraper.VirtualsScraper
	// Add more utilities here as needed, for example:
	// DataAnalyzer *analyzer.DataAnalyzer
	// ImageProcessor *processor.ImageProcessor
}

// NewUtilsManager creates and initializes all utilities
func NewUtilsManager(logger *log.Logger) *UtilsManager {
	if logger == nil {
		logger = log.New(os.Stdout, "[Utils] ", log.LstdFlags)
	}
	
	return &UtilsManager{
		logger:  logger,
		Scraper: webscraper.NewVirtualsScraper(logger),
		// Initialize other utilities here
	}
}

// Initialize sets up all required directories and configurations
func (m *UtilsManager) Initialize() error {
	// Create required directories
	if err := os.MkdirAll("training_data", 0755); err != nil {
		return fmt.Errorf("failed to create training_data directory: %v", err)
	}
	
	m.logger.Println("Utils Manager initialized successfully")
	return nil
}

// GetScraper returns the VirtualsScraper instance
func (m *UtilsManager) GetScraper() *webscraper.VirtualsScraper {
	return m.Scraper
}
