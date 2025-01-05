package storage

import (
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "sync"
    "time"
    "anondd/utils/models"
)

// AgentStore handles agent data storage
type AgentStore struct {
    BaseDir    string
    indexMutex sync.RWMutex
    logger     *log.Logger
}

// NewAgentStore creates a new agent store
func NewAgentStore(baseDir string, logger *log.Logger) *AgentStore {
    return &AgentStore{
        BaseDir: baseDir,
        logger:  logger,
    }
}

// SaveAgent saves an individual agent to storage
func (s *AgentStore) SaveAgent(agent *models.Agent) error {
    if agent.ID == "" {
        agent.GenerateID()
    }

    filePath := filepath.Join(s.BaseDir, "agents", fmt.Sprintf("%s.json", agent.ID))
    data, err := json.MarshalIndent(agent, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal agent: %w", err)
    }

    if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }

    return os.WriteFile(filePath, data, 0644)
}

// SaveAgents saves multiple agents and updates the index
func (s *AgentStore) SaveAgents(agents []models.Agent) error {
    for _, agent := range agents {
        if err := s.SaveAgent(&agent); err != nil {
            s.logger.Printf("Error saving agent %s: %v", agent.ID, err)
            continue
        }
    }
    return s.UpdateIndex(agents)
}

// UpdateIndex updates the agent index file
func (s *AgentStore) UpdateIndex(agents []models.Agent) error {
    s.indexMutex.Lock()
    defer s.indexMutex.Unlock()

    index := models.AgentIndex{
        LastUpdated: time.Now(),
        Agents:      make([]models.AgentSummary, len(agents)),
    }

    for i, agent := range agents {
        index.Agents[i] = models.AgentSummary{
            ID:    agent.ID,
            Name:  agent.Name,
            Price: agent.Price,
        }
    }

    data, err := json.MarshalIndent(index, "", "  ")
    if (err != nil) {
        return fmt.Errorf("failed to marshal index: %w", err)
    }

    indexPath := filepath.Join(s.BaseDir, "agent_index.json")
    return os.WriteFile(indexPath, data, 0644)
}

// GetAgent retrieves an agent by ID
func (s *AgentStore) GetAgent(id string) (*models.Agent, error) {
    filePath := filepath.Join(s.BaseDir, "agents", fmt.Sprintf("%s.json", id))
    data, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read agent file: %w", err)
    }

    var agent models.Agent
    if err := json.Unmarshal(data, &agent); err != nil {
        return nil, fmt.Errorf("failed to unmarshal agent: %w", err)
    }

    return &agent, nil
}

// GetIndex retrieves the current agent index
func (s *AgentStore) GetIndex() (*models.AgentIndex, error) {
    s.indexMutex.RLock()
    defer s.indexMutex.RUnlock()

    indexPath := filepath.Join(s.BaseDir, "agent_index.json")
    data, err := os.ReadFile(indexPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read index file: %w", err)
    }

    var index models.AgentIndex
    if err := json.Unmarshal(data, &index); err != nil {
        return nil, fmt.Errorf("failed to unmarshal index: %w", err)
    }

    return &index, nil
}
