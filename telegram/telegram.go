package telegram

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath" // Add this import
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"anondd/llm"
	"anondd/utils"
	"anondd/utils/models"
	"anondd/utils/storage"
)

// StartBot starts the Telegram bot with utils manager support.
func StartBot(ctx context.Context, botToken string, openRouterClient *llm.OpenRouterClient, utils *utils.UtilsManager, logger *log.Logger) error {
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

	// Process incoming updates (messages) until context is cancelled
	for {
		select {
		case update := <-updates:
			if update.Message != nil {
				handleCommand(bot, update, utils, openRouterClient, logger)
			}
		case <-ctx.Done():
			logger.Println("Shutting down Telegram bot...")
			return nil
		}
	}
}

func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, utilsManager *utils.UtilsManager, openRouterClient *llm.OpenRouterClient, logger *log.Logger) {
	message := update.Message
	parts := strings.Fields(message.Text)
	command := parts[0]

	// Get store from utils manager
	store := utilsManager.GetStore()

	switch command {
	case "/scrape_agents":
		handleScrapeAgents(bot, update, store, openRouterClient, logger)
	case "/give_dd":
		if len(parts) > 1 {
			if agentID, err := strconv.Atoi(parts[1]); err == nil {
				handleAgentDDScreenshot(bot, update, store, openRouterClient, agentID, logger)
			} else {
				handleAgentDD(bot, update, store, openRouterClient, strings.Join(parts[1:], " "), logger)
			}
		} else {
			handleRandomAgentDD(bot, update, store, openRouterClient, logger)
		}
	default:
		handleRegularMessage(bot, update, openRouterClient, logger)
	}
}

func handleScrapeAgents(bot *tgbotapi.BotAPI, update tgbotapi.Update, store *storage.AgentStore, client *llm.OpenRouterClient, logger *log.Logger) {
	chatID := update.Message.Chat.ID

	msg := tgbotapi.NewMessage(chatID, "🔍 Analyzing stored agent data...")
	bot.Send(msg)

	index, err := store.GetIndex()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Error accessing agent data"))
		return
	}

	var agentInfo strings.Builder
	agentInfo.WriteString("Current Agents Overview:\n\n")

	for _, summary := range index.Agents {
		if agent, err := store.GetAgent(summary.ID); err == nil {
			agentInfo.WriteString(fmt.Sprintf("Name: %s\nPrice: %s\nStats: %s\n\n",
				agent.Name, agent.Price, agent.Stats))
		}
	}

	ctx := context.Background()
	prompt := fmt.Sprintf("Analyze these AI agents and give a brief market analysis: %s", agentInfo.String())
	analysis, err := client.GetResponse(ctx, "custom", prompt)
	if err != nil {
		logger.Printf("Error getting AI analysis: %v", err)
		analysis = "Unable to analyze agents at this time."
	}

	response := fmt.Sprintf("📊 Found %d agents\n\n%s", len(index.Agents), analysis)
	bot.Send(tgbotapi.NewMessage(chatID, response))
}

func handleAgentDD(bot *tgbotapi.BotAPI, update tgbotapi.Update, store *storage.AgentStore, client *llm.OpenRouterClient, agentName string, logger *log.Logger) {
	chatID := update.Message.Chat.ID

	index, err := store.GetIndex()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Error accessing agent data"))
		return
	}

	var targetAgent *models.Agent
	for _, summary := range index.Agents {
		if strings.Contains(strings.ToLower(summary.Name), strings.ToLower(agentName)) {
			if agent, err := store.GetAgent(summary.ID); err == nil {
				targetAgent = agent
				break
			}
		}
	}

	if targetAgent == nil {
		bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ No agent found matching '%s'", agentName)))
		return
	}

	prompt := fmt.Sprintf("Analyze this AI agent in detail:\nName: %s\nPrice: %s\nStats: %s\nDescription: %s",
		targetAgent.Name, targetAgent.Price, targetAgent.Stats, targetAgent.Description)

	analysis, err := client.GetResponse(context.Background(), "agent_analysis", prompt)
	if err != nil {
		logger.Printf("Error getting agent analysis: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "Unable to analyze agent at this time."))
		return
	}

	response := fmt.Sprintf("🤖 Analysis for %s:\n\n%s", targetAgent.Name, analysis)
	bot.Send(tgbotapi.NewMessage(chatID, response))
}

func handleAgentDDScreenshot(bot *tgbotapi.BotAPI, update tgbotapi.Update, store *storage.AgentStore, client *llm.OpenRouterClient, agentID int, logger *log.Logger) {
	chatID := update.Message.Chat.ID

	// Loading texts
	loadingTexts := []string{
		"🔍 Scouting the digital jungle... 🌴🦜 Hang on while I swing through the data!",
		"🤖 Summoning the code wizards... 🧙‍♂️✨ Casting spells on the data!",
		"🚀 Launching into cyberspace... 🌌🔭 Preparing for a galactic search!",
		"👾 Battling digital gremlins... ⚔️👹 One sec while I vanquish these bugs!",
		"📡 Tuning into the Matrix... 🎛️🔮 Decoding the secrets for you!",
		"🌀 Diving into the data vortex... 🌊🤿 Surfacing with the details soon!",
		"⚡ Powering up the flux capacitor... ⏳⚙️ Time traveling for answers!",
		"🚦 Fastening seatbelts for the data rollercoaster... 🎢🔎 Hold tight!",
		"🧬 Unraveling the digital DNA... 🧪🔍 Piecing together the info puzzle!",
		"🎩 Abracadabra, data please... 🃏✨ Pulling magic answers out of the hat!",
	}

	// Select a random loading text
	rand.Seed(time.Now().UnixNano())
	loadingText := loadingTexts[rand.Intn(len(loadingTexts))]

	// Send loader message
	loaderMsg := tgbotapi.NewMessage(chatID, loadingText)
	loaderMsgID, _ := bot.Send(loaderMsg)

	// Get a random screenshot from the training_data/raw/debug directory
	debugDir := "training_data/raw/debug"
	files, err := os.ReadDir(debugDir)
	if err != nil {
		logger.Printf("Error reading debug directory: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Unable to read debug directory."))
		return
	}

	var screenshots []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".png") {
			screenshots = append(screenshots, filepath.Join(debugDir, file.Name()))
		}
	}

	if len(screenshots) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "❌ No screenshots available in debug directory."))
		return
	}

	// Select a random screenshot
	randomScreenshot := screenshots[rand.Intn(len(screenshots))]

	// Edit loader message to indicate screenshot is ready
	editMsg := tgbotapi.NewEditMessageText(chatID, loaderMsgID.MessageID, "✅ Agent details fetched successfully!")
	bot.Send(editMsg)

	// Send the screenshot to the user
	photo := tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(randomScreenshot))
	bot.Send(photo)

	// Add some light fun to the DD
	funMessage := fmt.Sprintf("Here's a sneak peek of agent %d! 🤖\n\n", agentID)
	funMessage += "Did you know? This agent is known for its exceptional performance and unique characteristics. Keep an eye on it! 👀"

	bot.Send(tgbotapi.NewMessage(chatID, funMessage))
}

func handleRandomAgentDD(bot *tgbotapi.BotAPI, update tgbotapi.Update, store *storage.AgentStore, client *llm.OpenRouterClient, logger *log.Logger) {
	// Pick a random agent ID between 0 and 100
	rand.Seed(time.Now().UnixNano())
	agentID := rand.Intn(101)

	handleAgentDDScreenshot(bot, update, store, client, agentID, logger)
}

func handleTopAgentsDD(bot *tgbotapi.BotAPI, update tgbotapi.Update, store *storage.AgentStore, client *llm.OpenRouterClient, logger *log.Logger) {
	chatID := update.Message.Chat.ID

	index, err := store.GetIndex()
	if err != nil {
		bot.Send(tgbotapi.NewMessage(chatID, "Error accessing agent data"))
		return
	}

	if len(index.Agents) == 0 {
		bot.Send(tgbotapi.NewMessage(chatID, "No agents data available."))
		return
	}

	var agentInfo strings.Builder
	agentInfo.WriteString("Top Agents Overview:\n\n")

	for i, summary := range index.Agents[:min(5, len(index.Agents))] {
		agentInfo.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, summary.Name, summary.Price))
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

	parts := strings.SplitN(userQuery, " ", 2)
	promptKey := "default"
	if len(parts) > 1 {
		promptKey = parts[0]
		userQuery = parts[1]
	}

	openRouterResponse, err := client.GetResponse(ctx, promptKey, userQuery)
	if err != nil {
		logger.Printf("Error retrieving response from OpenRouter: %v", err)
		openRouterResponse = "I'm sorry, something went wrong while processing your request."
	}

	reply := tgbotapi.NewMessage(update.Message.Chat.ID, openRouterResponse)
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
