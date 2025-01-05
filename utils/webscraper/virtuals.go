package webscraper

import (
    "fmt"
	"encoding/json"
    "log"
    "strings"
    "time"
    "path/filepath"
    "os"
    "context"
    "github.com/chromedp/chromedp"
    "github.com/PuerkitoBio/goquery"
    "anondd/utils/models"
    "anondd/utils/storage"
    "github.com/robfig/cron/v3"
    "sync"
    "io"
)

const (
    startAgentID = 1
    maxAgentID   = 500  // Increase range to catch more agents
    rawDataDir   = "training_data/raw"
    logFile      = "training_data/scraper.log"
)

type VirtualsScraper struct {
    baseURL   string
    logger    *log.Logger
    store     *storage.AgentStore
    scheduler *cron.Cron
    cache     struct {
        agents    []models.Agent
        lastFetch time.Time
        mu        sync.RWMutex
    }
}

// GetStore returns the store instance
func (v *VirtualsScraper) GetStore() *storage.AgentStore {
    return v.store
}

// NewVirtualsScraper initializes a new scraper for app.virtuals.io
func NewVirtualsScraper(logger *log.Logger, store *storage.AgentStore) *VirtualsScraper {
    if store == nil {
        logger.Fatal("store cannot be nil")
    }
    
    vs := &VirtualsScraper{
        baseURL:   "https://app.virtuals.io",
        logger:    logger,
        store:     store,
        scheduler: cron.New(),
    }
    
    // Set up the scheduler to run every 5 minutes
    if _, err := vs.scheduler.AddFunc("*/1 * * * *", func() {
        vs.logger.Println("Starting scheduled scrape...")
        if err := vs.ScrapeAgents(); err != nil {
            vs.logger.Printf("Scheduled scrape failed: %v", err)
        }
    }); err != nil {
        logger.Printf("Error setting up scheduler: %v", err)
    }
    
    // Start the scheduler
    vs.scheduler.Start()
    
    return vs
}

// ScrapeAgents fetches and processes all agent data
func (v *VirtualsScraper) ScrapeAgents() error {
    v.logger.Printf("[SCRAPE] Starting new scrape cycle")
    v.logger.Printf("[SCRAPE] Scanning agent IDs from %d to %d", startAgentID, maxAgentID)

    // Create scraper log file
    f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        v.logger.Printf("[ERROR] Could not open scraper log file: %v", err)
    } else {
        defer f.Close()
        // Add file logging while keeping console logging
        v.logger.SetOutput(io.MultiWriter(os.Stdout, f))
    }

    // Ensure raw data directory exists
    if err := os.MkdirAll(rawDataDir, 0755); err != nil {
        return fmt.Errorf("[ERROR] failed to create raw data directory: %w", err)
    }

    var agents []models.Agent
    successCount := 0
    errorCount := 0

    // Iterate through agent IDs
    for id := startAgentID; id <= maxAgentID; id++ {
        agentID := fmt.Sprintf("%d", id)
        
        // Check if we should fetch this agent
        if (!v.store.ShouldFetch(agentID)) {
            v.logger.Printf("[SKIP] Agent %s was recently fetched", agentID)
            continue
        }

        endpoint := fmt.Sprintf("/virtuals/%d", id)
        v.logger.Printf("[FETCH] Attempting to fetch agent %d from %s", id, endpoint)

        // Fetch HTML using chromedp
        doc, err := v.FetchHTML(endpoint)
        if err != nil {
            errorCount++
            v.logger.Printf("[ERROR] Failed to fetch HTML for ID %d: %v", id, err)
            continue
        }

        // Parse HTML
        agent, err := v.parseAgentPage(doc, id)
        if err != nil {
            errorCount++
            v.logger.Printf("[ERROR] Failed to parse HTML for ID %d: %v", id, err)
            continue
        }

        if agent != nil {
            // Mark as fetched regardless of status
            v.store.MarkFetched(agentID)
            
            successCount++
            agents = append(agents, *agent)
            v.logger.Printf("[SUCCESS] Successfully processed agent %d: %s (Status: %s)", 
                id, agent.Name, agent.Status)
        }

        // Add delay to avoid rate limiting
        v.logger.Printf("[DELAY] Waiting 500ms before next request")
        time.Sleep(500 * time.Millisecond)
    }

    // Log summary
    v.logger.Printf("[SUMMARY] Scrape cycle completed:")
    v.logger.Printf("- Total attempts: %d", maxAgentID-startAgentID+1)
    v.logger.Printf("- Successful: %d", successCount)
    v.logger.Printf("- Failed: %d", errorCount)
    v.logger.Printf("- Agents found: %d", len(agents))

    if len(agents) > 0 {
        if err := v.store.UpdateIndex(agents); err != nil {
            v.logger.Printf("[ERROR] Failed to update index: %v", err)
        } else {
            v.logger.Printf("[SUCCESS] Updated index with %d agents", len(agents))
        }
    }

    return nil
}

func (v *VirtualsScraper) FetchHTML(endpoint string) (*goquery.Document, error) {
    url := v.baseURL + endpoint
    v.logger.Printf("[DEBUG] Fetching URL: %s", url)

    // Create Chrome instance with options
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("headless", true),
        chromedp.Flag("disable-gpu", true),
        chromedp.Flag("no-sandbox", true),
        chromedp.Flag("disable-dev-shm-usage", true),
        chromedp.Flag("disable-web-security", true),
        chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"),
    )

    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
    defer cancel()

    ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(v.logger.Printf))
    defer cancel()

    // Increase timeout to 60 seconds
    ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
    defer cancel()

    var htmlContent string
    var debugScreenshot []byte
    var pageTitle string

    // Add error channel for monitoring
    errChan := make(chan error, 1)
    doneChan := make(chan bool, 1)

    go func() {
        err := chromedp.Run(ctx,
            chromedp.Navigate(url),
            chromedp.WaitVisible(`body`, chromedp.ByQuery), // Changed from #root to body
            chromedp.Sleep(5*time.Second),
            chromedp.CaptureScreenshot(&debugScreenshot),
            chromedp.Title(&pageTitle),
            chromedp.OuterHTML(`html`, &htmlContent, chromedp.ByQuery),
        )
        if err != nil {
            errChan <- err
            return
        }
        doneChan <- true
    }()

    // Wait for completion or error
    select {
    case err := <-errChan:
        v.logger.Printf("[ERROR] Chrome task failed: %v", err)
        return nil, fmt.Errorf("chrome automation failed: %w", err)
    case <-doneChan:
        v.logger.Printf("[SUCCESS] Page loaded successfully: %s", pageTitle)
    case <-time.After(55*time.Second):
        v.logger.Printf("[ERROR] Timeout while loading page")
        return nil, fmt.Errorf("timeout while loading page")
    }

    // Debug logging
    v.logger.Printf("[DEBUG] Page title: %s", pageTitle)
    v.logger.Printf("[DEBUG] Content length: %d bytes", len(htmlContent))

    // Save debug data
    debugDir := filepath.Join(rawDataDir, "debug")
    if err := os.MkdirAll(debugDir, 0755); err == nil {
        timestamp := time.Now().Unix()
        
        // Save screenshot
        screenshotPath := filepath.Join(debugDir, fmt.Sprintf("screenshot_%s_%d.png",
            strings.TrimPrefix(endpoint, "/virtuals/"), timestamp))
        if err := os.WriteFile(screenshotPath, debugScreenshot, 0644); err != nil {
            v.logger.Printf("[WARN] Failed to save screenshot: %v", err)
        }

        // Save HTML
        htmlPath := filepath.Join(debugDir, fmt.Sprintf("page_%s_%d.html",
            strings.TrimPrefix(endpoint, "/virtuals/"), timestamp))
        if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
            v.logger.Printf("[WARN] Failed to save HTML: %v", err)
        }
    }

    return goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
}

// Add helper function to parse selectors
func (v *VirtualsScraper) extractTextBySelector(doc *goquery.Document, selectors map[string][]string) map[string]string {
    result := make(map[string]string)
    
    for field, selectorList := range selectors {
        for _, selector := range selectorList {
            doc.Find(selector).Each(func(i int, s *goquery.Selection) {
                text := strings.TrimSpace(s.Text())
                if text != "" {
                    v.logger.Printf("[DEBUG] Found %s using selector '%s': %s", field, selector, text)
                    if _, exists := result[field]; !exists {
                        result[field] = text
                    }
                }
            })
        }
    }
    return result
}

func (v *VirtualsScraper) parseAgentPage(doc *goquery.Document, id int) (*models.Agent, error) {
    v.logger.Printf("[DEBUG] Starting to parse agent page %d", id)
    
    // Save raw HTML first
    rawPath := filepath.Join(rawDataDir, fmt.Sprintf("agent_%d_raw.html", id))
    if html, err := doc.Html(); err == nil {
        if err := os.WriteFile(rawPath, []byte(html), 0644); err != nil {
            v.logger.Printf("[WARN] Failed to save raw HTML: %v", err)
        }
    }

    // Define selectors for different fields
    selectors := map[string][]string{
        "name": {
            ".text-neutral10.text-2xl",
            "h1",
            ".agent-name",
            "div.text-2xl",
        },
        "price": {
            ".text-neutral30",
            "div:contains('$')",
            ".price",
        },
        "description": {
            "div:contains('Biography') + div",
            ".text-base.text-neutral30.break-all",
            ".agent-description",
        },
    }

    // Extract text using selectors
    extracted := v.extractTextBySelector(doc, selectors)
    
    // Log all found text for debugging
    v.logger.Printf("[DEBUG] Extracted data for agent %d:", id)
    for field, value := range extracted {
        v.logger.Printf("[DEBUG] %s: %s", field, value)
    }

    // Create agent with found data
    agent := &models.Agent{
        ScrapedAt:    time.Now(),
        ParseSuccess: true,
    }

    // Extract influence metrics
    metrics := v.extractInfluenceMetrics(doc)
    v.logger.Printf("[DEBUG] Extracted metrics: %+v", metrics)

    // Extract token data
    tokenData := v.extractTokenData(doc)
    v.logger.Printf("[DEBUG] Extracted token data: %+v", tokenData)

    // Set agent fields
    agent.Name = extracted["name"]
    agent.Price = extracted["price"]
    agent.Description = extracted["description"]
    agent.InfluenceMetrics = metrics
    agent.TokenData = tokenData

    // Save parsed data as JSON
    if agent.Name != "" || agent.Price != "" || agent.Description != "" {
        jsonPath := filepath.Join(rawDataDir, fmt.Sprintf("agent_%d.json", id))
        if data, err := json.MarshalIndent(agent, "", "  "); err == nil {
            if err := os.WriteFile(jsonPath, data, 0644); err != nil {
                v.logger.Printf("[WARN] Failed to save JSON data: %v", err)
            }
        }
    }

    if agent.Name == "" {
        v.logger.Printf("[WARN] No name found for agent %d, attempting deeper search", id)
        // Try to find any meaningful text
        doc.Find("div,span,p").Each(func(i int, s *goquery.Selection) {
            text := strings.TrimSpace(s.Text())
            if strings.Contains(text, "chan") || strings.Contains(text, "Agent") {
                v.logger.Printf("[DEBUG] Potential name found: %s", text)
            }
        })
        return nil, fmt.Errorf("no agent name found for ID %d", id)
    }

    agent.GenerateID()
    agent.UpdateStatus()
    
    // Log final parsed agent
    v.logger.Printf("[SUCCESS] Parsed agent %d: %+v", id, agent)
    return agent, nil
}

func (v *VirtualsScraper) extractText(doc *goquery.Document, selectors []string) string {
    for _, selector := range selectors {
        if text := strings.TrimSpace(doc.Find(selector).First().Text()); text != "" {
            return text
        }
    }
    return ""
}

func (v *VirtualsScraper) extractInfluenceMetrics(doc *goquery.Document) models.InfluenceMetrics {
    var metrics models.InfluenceMetrics
    
    doc.Find("div:contains('Influence Metrics')").Parent().Find(".rounded-2xl").Each(func(i int, s *goquery.Selection) {
        label := strings.TrimSpace(s.Find(".text-neutral50").Text())
        value := strings.TrimSpace(s.Find(".text-neutral10").Text())
        
        switch strings.ToLower(label) {
        case "mindshare":
            metrics.Mindshare = value
        case "impressions":
            metrics.Impressions = value
        case "engagement":
            metrics.Engagement = value
        case "followers":
            metrics.Followers = value
        case "smart followers":
            metrics.SmartFollowers = value
        case "top tweets":
            metrics.TopTweets = value
        }
    })
    
    return metrics
}

func (v *VirtualsScraper) extractTokenData(doc *goquery.Document) models.TokenData {
    var tokenData models.TokenData
    
    doc.Find("div:contains('Token Data')").Parent().Find(".grid-cols-4").Each(func(i int, s *goquery.Selection) {
        s.Find(".flex-col").Each(func(j int, col *goquery.Selection) {
            label := strings.TrimSpace(col.Find(".text-neutral50").Text())
            value := strings.TrimSpace(col.Find(".text-[#236D66]").Text())
            
            switch strings.ToLower(label) {
            case "mc (fdv)":
                tokenData.MCFDV = value
            case "24h chg":
                tokenData.Change24h = value
            case "tvl":
                tokenData.TVL = value
            case "holders":
                tokenData.Holders = value
            case "24h vol":
                tokenData.Volume24h = value
            case "inferences":
                tokenData.Inferences = value
            }
        })
    })
    
    return tokenData
}

func (v *VirtualsScraper) logElementsForDebugging(doc *goquery.Document) {
    v.logger.Println("[DEBUG] Element structure:")
    doc.Find("*").Each(func(i int, s *goquery.Selection) {
        if class, exists := s.Attr("class"); exists {
            text := strings.TrimSpace(s.Text())
            if text != "" {
                v.logger.Printf("[DEBUG] Element %d - Class: '%s', Text preview: '%s'",
                    i, class, truncateString(text, 50))
            }
        }
    })
}

func truncateString(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[:n] + "..."
}

// StopScheduler implements the Scraper interface
func (v *VirtualsScraper) StopScheduler() {
    if v.scheduler != nil {
        v.scheduler.Stop()
    }
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
