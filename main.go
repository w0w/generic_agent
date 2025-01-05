package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "anondd/api"
    "anondd/llm"
    "anondd/telegram"
    "anondd/utils"
)

func main() {
    logger := log.New(os.Stdout, "[anondd] ", log.LstdFlags|log.Lshortfile)

    // Initialize utils manager
    logger.Println("Initializing utils manager...")
    utilsManager := utils.NewUtilsManager(logger)
    if err := utilsManager.Initialize(); err != nil {
        logger.Fatalf("Failed to initialize utils: %v", err)
    }
    logger.Println("Utils manager initialized successfully")

    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Handle shutdown signals
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        logger.Println("Received shutdown signal, shutting down gracefully...")
        utilsManager.GetScraper().StopScheduler()
        cancel()
    }()

    // Get environment variables
    logger.Println("Fetching environment variables...")
    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")

    if botToken == "" || openRouterAPIKey == "" {
        logger.Fatal("Please set TELEGRAM_BOT_TOKEN and OPENROUTER_API_KEY environment variables")
    }
    logger.Println("Environment variables fetched successfully")

    openRouterClient := llm.NewOpenRouterClient(openRouterAPIKey, "https://openrouter.ai/api/v1/chat/completions", logger)

    // Initialize API server - use GetStore instead of accessing Store directly
    logger.Println("Initializing API server...")
    apiServer := api.NewAPIServer(utilsManager.GetStore(), logger)
    apiServer.SetupRoutes()
    logger.Println("API server initialized successfully")

    // Start HTTP server in a goroutine with context
    srv := &http.Server{
        Addr:    ":8080",
        Handler: http.DefaultServeMux,
    }
    
    go func() {
        logger.Println("Starting HTTP server on port 8080...")
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            logger.Printf("API server error: %v", err)
        }
    }()

    // Shutdown on context cancellation
    go func() {
        <-ctx.Done()
        logger.Println("Shutting down HTTP server...")
        if err := srv.Shutdown(context.Background()); err != nil {
            logger.Printf("HTTP server shutdown error: %v", err)
        }
    }()

    // Start the bot with context
    logger.Println("Starting Telegram bot...")
    if err := telegram.StartBot(ctx, botToken, openRouterClient, utilsManager, logger); err != nil {
        logger.Fatalf("Failed to start Telegram bot: %v", err)
    }
    logger.Println("Telegram bot started successfully")
}
