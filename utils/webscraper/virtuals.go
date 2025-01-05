package webscraper

import (
    "fmt"
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
    startAgentID = 235
    maxAgentID   = 240  // Reduced range for testing
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
            successCount++
            agents = append(agents, *agent)
            v.logger.Printf("[SUCCESS] Successfully processed agent %d: %s", id, agent.Name)
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

func (v *VirtualsScraper) parseAgentPage(doc *goquery.Document, id int) (*models.Agent, error) {
    v.logger.Printf("[DEBUG] Starting to parse agent page %d", id)
    
    agent := &models.Agent{
        ScrapedAt: time.Now(),
    }

    // Debug HTML structure before parsing
    if html, err := doc.Html(); err == nil {
        debugPath := filepath.Join(rawDataDir, "debug", fmt.Sprintf("parsed_%d_%d.html", id, time.Now().Unix()))
        if err := os.WriteFile(debugPath, []byte(html), 0644); err != nil {
            v.logger.Printf("[WARN] Failed to save parsed HTML: %v", err)
        }
    }

    // Extract name (using the heading format shown)
    name := strings.TrimSpace(doc.Find("h1").First().Text())
    if name == "" {
        // Try alternative selector
        name = strings.TrimSpace(doc.Find(".MuiTypography-h1").First().Text())
    }

    // Extract price (from the $REIKO format shown)
    price := doc.Find("div:contains('$')").First().Text()
    price = strings.TrimSpace(strings.Split(price, "\n")[0]) // Take first line only

    // Extract stats from Influence Metrics section
    var stats strings.Builder
    doc.Find("div:contains('Influence Metrics')").Parent().Find("div").Each(func(i int, s *goquery.Selection) {
        text := strings.TrimSpace(s.Text())
        if text != "" && !strings.Contains(text, "Influence Metrics") {
            stats.WriteString(text + "\n")
        }
    })

    // Extract description from Biography section
    description := doc.Find("div:contains('Biography')").Parent().Find("div").Last().Text()
    description = strings.TrimSpace(description)

    // Additional token data extraction
    doc.Find("div:contains('Token Data')").Parent().Find("div").Each(func(i int, s *goquery.Selection) {
        text := strings.TrimSpace(s.Text())
        if text != "" && strings.Contains(text, "$") {
            stats.WriteString("Token: " + text + "\n")
        }
    })

    // Log extracted data
    v.logger.Printf("[DEBUG] Extracted data for agent %d:", id)
    v.logger.Printf("  Name: %s", name)
    v.logger.Printf("  Price: %s", price)
    v.logger.Printf("  Description: %s", description)
    v.logger.Printf("  Stats: %s", stats.String())

    // Save the data
    agent.Name = name
    agent.Price = price
    agent.Description = description
    agent.Stats = stats.String()

    if agent.Name == "" {
        v.logger.Printf("[ERROR] No agent name found for ID %d", id)
        doc.Find("*").Each(func(i int, s *goquery.Selection) {
            if class, exists := s.Attr("class"); exists {
                text := strings.TrimSpace(s.Text())
                if text != "" {
                    v.logger.Printf("[DEBUG] Element %d - Class: '%s', Text: '%s'", i, class, text)
                }
            }
        })
        return nil, fmt.Errorf("no agent name found for ID %d", id)
    }

    agent.GenerateID()
    return agent, nil
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
