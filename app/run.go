package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"main/actions"
	"main/handlers"
	"os"
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

	act := handlers.ActiveHandlers{Handlers: []handlers.Handler{
		// Place your handlers here
		handlers.CommandHandler.Product(actions.SayHi{Name: "start-cmd", Client: bot}, []handlers.Filter{startFilter}),
	}}

	return act
}

func main() {
	client := connect(true)
	act := getBotActions(*client)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := client.GetUpdatesChan(updateConfig)
	for update := range updates {
		_ = act.HandleAll(update)
	}
}
