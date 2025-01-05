package models

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"
)

// Agent represents a single agent with all its details
type Agent struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Stats       string    `json:"stats"`
    Price       string    `json:"price"`
    ScrapedAt   time.Time `json:"scraped_at"`
}

// AgentIndex represents the index of all agents
type AgentIndex struct {
    LastUpdated time.Time     `json:"last_updated"`
    Agents      []AgentSummary `json:"agents"`
}

// AgentSummary represents basic agent info for the index
type AgentSummary struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Price string `json:"price"`
}

// GenerateID creates a unique ID for an agent
func (a *Agent) GenerateID() {
    hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%s", a.Name, a.Price)))
    a.ID = hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter ID
}

// Validate checks if the agent data is valid
func (a *Agent) Validate() error {
    if a.Name == "" {
        return fmt.Errorf("agent name is required")
    }
    if a.ID == "" {
        a.GenerateID()
    }
    return nil
}

// ToSummary converts an Agent to AgentSummary
func (a *Agent) ToSummary() AgentSummary {
    return AgentSummary{
        ID:    a.ID,
        Name:  a.Name,
        Price: a.Price,
    }
}

// AgentData represents the raw scraped data
type AgentData struct {
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Stats       string    `json:"stats"`
    Price       string    `json:"price"`
    ScrapedAt   time.Time `json:"scraped_at"`
}

// FromAgentData converts AgentData to Agent model
func FromAgentData(data *AgentData) *Agent {
    agent := &Agent{
        Name:        data.Name,
        Description: data.Description,
        Stats:       data.Stats,
        Price:       data.Price,
        ScrapedAt:   data.ScrapedAt,
    }
    agent.GenerateID()
    return agent
}
