package models

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strings"
    "time"
)

const (
    StatusDefault = "default"
    StatusActive  = "active"
    StatusDead    = "dead"
    StatusLatent  = "latent"
)

type InfluenceMetrics struct {
    Mindshare      string `json:"mindshare"`
    Impressions    string `json:"impressions"`
    Engagement     string `json:"engagement"`
    Followers      string `json:"followers"`
    SmartFollowers string `json:"smart_followers"`
    TopTweets      string `json:"top_tweets"`
}

type TokenData struct {
    MCFDV       string `json:"mc_fdv"`
    Change24h   string `json:"change_24h"`
    TVL         string `json:"tvl"`
    Holders     string `json:"holders"`
    Volume24h   string `json:"volume_24h"`
    Inferences  string `json:"inferences"`
}

// Agent represents a single agent with all its details
type Agent struct {
    ID              string          `json:"id"`
    Name            string          `json:"name"`
    Description     string          `json:"description"`
    Stats           string          `json:"stats"`
    Price           string          `json:"price"`
    ScrapedAt       time.Time       `json:"scraped_at"`
    Status          string          `json:"status"`
    LastChecked     time.Time       `json:"last_checked"`
    UpdateCount     int             `json:"update_count"`
    InfluenceMetrics InfluenceMetrics `json:"influence_metrics"`
    TokenData        TokenData        `json:"token_data"`
    LastError        string          `json:"last_error,omitempty"`
    ParseSuccess     bool            `json:"parse_success"`
    RetryCount      int             `json:"retry_count"`
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

// ValidateAndClean checks and cleans agent data
func (a *Agent) ValidateAndClean() {
    // Clean name
    a.Name = strings.TrimSpace(a.Name)
    if strings.HasPrefix(a.Name, "$") {
        parts := strings.Fields(a.Name)
        if len(parts) > 1 {
            a.Name = parts[0]
            if a.Price == "" && len(parts) > 1 {
                a.Price = parts[1]
            }
        }
    }

    // Clean price
    if strings.Contains(a.Price, "$") {
        a.Price = strings.TrimSpace(strings.Split(a.Price, "\n")[0])
    }

    // Clean description
    a.Description = strings.TrimSpace(a.Description)
    if len(a.Description) > 1000 {
        a.Description = a.Description[:1000] + "..."
    }
}

// ToSummary converts an Agent to AgentSummary
func (a *Agent) ToSummary() AgentSummary {
    return AgentSummary{
        ID:    a.ID,
        Name:  a.Name,
        Price: a.Price,
    }
}

// IsStale checks if the agent needs to be rechecked
func (a *Agent) IsStale(duration time.Duration) bool {
    return time.Since(a.LastChecked) > duration
}

// UpdateStatus determines the agent's status based on its data
func (a *Agent) UpdateStatus() {
    switch {
    case a.Price == "" && a.Description == "":
        a.Status = StatusDead
    case a.UpdateCount == 0:
        a.Status = StatusDefault
    case strings.Contains(strings.ToLower(a.Description), "inactive") || 
         strings.Contains(strings.ToLower(a.Description), "discontinued"):
        a.Status = StatusLatent
    default:
        a.Status = StatusActive
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

func (a *Agent) SetError(err error) {
    if err != nil {
        a.LastError = err.Error()
        a.ParseSuccess = false
    }
}
