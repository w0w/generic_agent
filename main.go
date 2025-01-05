package main

import (
    "log"
    "os"
    "anondd/llm"
    "anondd/telegram"
    "anondd/utils"
)

func main() {
    logger := log.New(os.Stdout, "[MyApp] ", log.LstdFlags|log.Lshortfile)

    // Initialize utils manager
    utilsManager := utils.NewUtilsManager(logger)
    if err := utilsManager.Initialize(); err != nil {
        logger.Fatalf("Failed to initialize utils: %v", err)
    }

    // Get environment variables
    botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
    openRouterAPIKey := os.Getenv("OPENROUTER_API_KEY")

    if botToken == "" || openRouterAPIKey == "" {
        logger.Fatal("Please set TELEGRAM_BOT_TOKEN and OPENROUTER_API_KEY environment variables")
    }

    openRouterClient := llm.NewOpenRouterClient(openRouterAPIKey, "https://openrouter.ai/api/v1/chat/completions", logger)

    // Start the bot with utils manager
    if err := telegram.StartBot(botToken, openRouterClient, utilsManager, logger); err != nil {
        logger.Fatalf("Failed to start Telegram bot: %v", err)
    }
}
