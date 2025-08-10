package main

import (
	"encoding/json"
	"log"
	"main/actions"
	"main/controllers"
	"main/database"
	"main/handlers"
	"main/util"
	"os"
	"slices"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func connect(debug bool) *tgbotapi.BotAPI {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("API_KEY"))
	if err != nil {
		panic(err)
	}

	bot.Debug = debug
	log.Printf("Successfully authorized on account @%s", bot.Self.UserName)

	return bot
}

func getBotActions(bot *tgbotapi.BotAPI) handlers.ActiveHandlers {
	startFilter := func(update tgbotapi.Update) bool { return update.Message.Command() == "start" }

	adminIdStr := os.Getenv("ADMIN_ID")
	adminId, err := strconv.ParseInt(adminIdStr, 10, 64)
	if err != nil {
		panic(err)
	}
	adminFilter := func(update tgbotapi.Update) bool { return util.GetMessage(update).Chat.ID == adminId }

	mainPageCallQuery := func(update tgbotapi.Update) bool {
		var data map[string]any
		err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

		return err == nil && slices.Contains([]string{"n", "p", "c"}, data["a"].(string))
		// Add and Secret actions will be handled in other actions
	}

	act := handlers.ActiveHandlers{Handlers: []handlers.Handler{
		handlers.CommandHandler.Product(actions.MainPage{Name: "main-page-cmd", Client: *bot}, []handlers.Filter{startFilter, adminFilter}),
		handlers.CallbackQueryHandler.Product(actions.MainPage{Name: "main-page-call-query", Client: *bot}, []handlers.Filter{mainPageCallQuery, adminFilter}),
	}}

	return act
}

func main() {
	_ = godotenv.Load()

	debug := os.Getenv("DEBUG") == "true"

	err := database.InitDb()
	if err != nil {
		panic(err)
	}

	log.Println("Database initialized successfully")

	client := connect(debug)
	act := getBotActions(client)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	go func() {
		for {
			time.Sleep(5 * time.Second)
			err := controllers.DeleteOldSessions()
			if err != nil {
				log.Println("Error deleting old sessions: ", err)
			}
		}
	}()

	stepManager := controllers.GetNextStepManager()

	updates := client.GetUpdatesChan(updateConfig)
	for update := range updates {
		_ = act.HandleAll(update)
		controllers.RunStepUpdates(update, stepManager, *client)
	}
}
