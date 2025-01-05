package api

import (
    "encoding/json"
    "log"
    "net/http"
    "anondd/utils/storage"
    "github.com/gorilla/mux"
)

type APIServer struct {
    store  *storage.AgentStore
    logger *log.Logger
}

func NewAPIServer(store *storage.AgentStore, logger *log.Logger) *APIServer {
    return &APIServer{
        store:  store,
        logger: logger,
    }
}

func (s *APIServer) SetupRoutes() {
    router := mux.NewRouter()

    // API routes
    router.HandleFunc("/api/agents", s.handleGetAllAgents).Methods("GET")
    router.HandleFunc("/api/agents/{id}", s.handleGetAgent).Methods("GET")
    router.HandleFunc("/api/index", s.handleGetIndex).Methods("GET")

    // Set router as default HTTP handler
    http.Handle("/", router)
    s.logger.Println("API routes set up successfully")
}

func (s *APIServer) handleGetAllAgents(w http.ResponseWriter, r *http.Request) {
    s.logger.Println("Received request to get all agents")
    index, err := s.store.GetIndex()
    if err != nil {
        http.Error(w, "Failed to retrieve agents", http.StatusInternalServerError)
        s.logger.Printf("Error getting agents: %v", err)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(index.Agents)
    s.logger.Println("Successfully retrieved all agents")
}

func (s *APIServer) handleGetAgent(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    id := vars["id"]
    s.logger.Printf("Received request to get agent with ID: %s", id)

    agent, err := s.store.GetAgent(id)
    if err != nil {
        http.Error(w, "Agent not found", http.StatusNotFound)
        s.logger.Printf("Error getting agent %s: %v", id, err)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(agent)
    s.logger.Printf("Successfully retrieved agent with ID: %s", id)
}

func (s *APIServer) handleGetIndex(w http.ResponseWriter, r *http.Request) {
    s.logger.Println("Received request to get agent index")
    index, err := s.store.GetIndex()
    if err != nil {
        http.Error(w, "Failed to retrieve index", http.StatusInternalServerError)
        s.logger.Printf("Error getting index: %v", err)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(index)
    s.logger.Println("Successfully retrieved agent index")
}
