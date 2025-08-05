package main

import (
	"log"
	"main/actions"
	"main/handlers"
	"os"
	"strconv"
	"strings"

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

func getBotActions(bot tgbotapi.BotAPI) handlers.ActiveHandlers {
	startFilter := func(update tgbotapi.Update) bool { return update.Message.Command() == "start" }

	adminIdStr := os.Getenv("ADMIN_ID")
	adminId, err := strconv.ParseInt(adminIdStr, 10, 64)
	if err != nil {
		panic(err)
	}
	adminFilter := func(update tgbotapi.Update) bool { return GetMessage(update).Chat.ID == adminId }

	mainPageCallQuery := func(update tgbotapi.Update) bool { return strings.HasPrefix(update.CallbackQuery.Data, "MP") }

	act := handlers.ActiveHandlers{Handlers: []handlers.Handler{
		handlers.CommandHandler.Product(actions.MainPage{Name: "main-page-cmd", Client: bot}, []handlers.Filter{startFilter, adminFilter}),
		handlers.CallbackQueryHandler.Product(actions.MainPage{Name: "main-page-call-query", Client: bot}, []handlers.Filter{mainPageCallQuery, adminFilter}),
	}}

	return act
}

func main() {
	_ = godotenv.Load()

	debug := os.Getenv("DEBUG") == "true"

	client := connect(debug)
	act := getBotActions(*client)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := client.GetUpdatesChan(updateConfig)
	for update := range updates {
		_ = act.HandleAll(update)
	}
}
