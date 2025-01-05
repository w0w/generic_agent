package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"anondd/llm"
	"anondd/utils"
	"anondd/utils/webscraper"
)

// StartBot starts the Telegram bot with utils manager support.
func StartBot(botToken string, openRouterClient *llm.OpenRouterClient, utils *utils.UtilsManager, logger *log.Logger) error {
	// Initialize the Telegram bot.
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return err
	}
	bot.Debug = true
	logger.Printf("Authorized on account %s", bot.Self.UserName)

	// Configure the update receiver.
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Process incoming updates (messages).
	for update := range updates {
		if update.Message != nil {
			handleCommand(bot, update, utils, openRouterClient, logger)
		}
	}
	return nil
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, utilsManager *utils.UtilsManager, openRouterClient *llm.OpenRouterClient, logger *log.Logger) {
    message := update.Message
    parts := strings.Fields(message.Text)
    command := parts[0]

    switch command {
    case "/scrape_agents":
        handleScrapeAgents(bot, update, utilsManager.GetScraper(), openRouterClient, logger)
    case "/give_dd":
        if len(parts) > 1 {
            handleAgentDD(bot, update, utilsManager.GetScraper(), openRouterClient, strings.Join(parts[1:], " "), logger)
        } else {
            handleTopAgentsDD(bot, update, utilsManager.GetScraper(), openRouterClient, logger)
        }
    default:
        handleRegularMessage(bot, update, openRouterClient, logger)
    }
}

func handleScrapeAgents(bot *tgbotapi.BotAPI, update tgbotapi.Update, scraper *webscraper.VirtualsScraper, client *llm.OpenRouterClient, logger *log.Logger) {
    chatID := update.Message.Chat.ID
    
    // Send starting message
    msg := tgbotapi.NewMessage(chatID, "🔍 Scraping agent data...")
    bot.Send(msg)

    // Fetch and parse data
    doc, err := scraper.FetchHTML("/agents")
    if (err != nil) {
        errorMsg := fmt.Sprintf("Error fetching agents: %v", err)
        bot.Send(tgbotapi.NewMessage(chatID, errorMsg))
        return
    }

    agents, err := scraper.ParseData(doc)
    if (err != nil) {
        errorMsg := fmt.Sprintf("Error parsing agents: %v", err)
        bot.Send(tgbotapi.NewMessage(chatID, errorMsg))
        return
    }

    // Prepare agent information for LLM
    var agentInfo strings.Builder
    agentInfo.WriteString("Here are the top agents:\n\n")
    for _, agent := range agents {
        agentInfo.WriteString(fmt.Sprintf("Name: %s\nPrice: %s\nStats: %s\n\n", 
            agent.Name, agent.Price, agent.Stats))
    }

    // Get AI analysis of the agents
    ctx := context.Background()
    prompt := fmt.Sprintf("Analyze these AI agents and give a brief market analysis: %s", agentInfo.String())
    analysis, err := client.GetResponse(ctx, "custom", prompt)
    if (err != nil) {
        logger.Printf("Error getting AI analysis: %v", err)
        analysis = "Unable to analyze agents at this time."
    }

    // Send combined response
    response := fmt.Sprintf("📊 Found %d agents\n\n%s", len(agents), analysis)
    bot.Send(tgbotapi.NewMessage(chatID, response))
}

func handleAgentDD(bot *tgbotapi.BotAPI, update tgbotapi.Update, scraper *webscraper.VirtualsScraper, client *llm.OpenRouterClient, agentName string, logger *log.Logger) {
    chatID := update.Message.Chat.ID
    
    agent := scraper.FindAgent(agentName)
    if agent == nil {
        bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ No agent found matching '%s'", agentName)))
        return
    }

    prompt := fmt.Sprintf("Analyze this AI agent in detail:\nName: %s\nPrice: %s\nStats: %s\nDescription: %s",
        agent.Name, agent.Price, agent.Stats, agent.Description)
    
    analysis, err := client.GetResponse(context.Background(), "agent_analysis", prompt)
    if err != nil {
        logger.Printf("Error getting agent analysis: %v", err)
        bot.Send(tgbotapi.NewMessage(chatID, "Unable to analyze agent at this time."))
        return
    }

    response := fmt.Sprintf("🤖 Analysis for %s:\n\n%s", agent.Name, analysis)
    bot.Send(tgbotapi.NewMessage(chatID, response))
}

func handleTopAgentsDD(bot *tgbotapi.BotAPI, update tgbotapi.Update, scraper *webscraper.VirtualsScraper, client *llm.OpenRouterClient, logger *log.Logger) {
    chatID := update.Message.Chat.ID
    
    agents := scraper.GetAgents()
    if len(agents) == 0 {
        bot.Send(tgbotapi.NewMessage(chatID, "No agents data available."))
        return
    }

    // Prepare top agents info
    var agentInfo strings.Builder
    agentInfo.WriteString("Top Agents Overview:\n\n")
    for i, agent := range agents[:min(5, len(agents))] {
        agentInfo.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, agent.Name, agent.Price))
    }

    analysis, err := client.GetResponse(context.Background(), "agent_analysis", agentInfo.String())
    if err != nil {
        logger.Printf("Error getting market analysis: %v", err)
        bot.Send(tgbotapi.NewMessage(chatID, "Unable to analyze market at this time."))
        return
    }

    response := fmt.Sprintf("📊 Market Analysis\n\n%s", analysis)
    bot.Send(tgbotapi.NewMessage(chatID, response))
}

func handleRegularMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, client *llm.OpenRouterClient, logger *log.Logger) {
	userQuery := update.Message.Text
	ctx := context.Background()

	// Parse user input for a task type
	parts := strings.SplitN(userQuery, " ", 2)
	promptKey := "default"
	if len(parts) > 1 {
		promptKey = parts[0]
		userQuery = parts[1]
	}

	// Send the user's query to OpenRouter with prompt injection.
	openRouterResponse, err := client.GetResponse(ctx, promptKey, userQuery)
	if err != nil {
		logger.Printf("Error retrieving response from OpenRouter: %v", err)
		openRouterResponse = "I'm sorry, something went wrong while processing your request."
	}

	// Create a reply with the OpenRouter response.
	reply := tgbotapi.NewMessage(update.Message.Chat.ID, openRouterResponse)

	// Send the reply back to the user.
	if _, err := bot.Send(reply); err != nil {
		logger.Printf("Error sending message: %v", err)
	}
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
