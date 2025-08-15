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
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var (
	currentBot *tgbotapi.BotAPI
	botMutex   sync.Mutex
)

func connect(debug bool) *tgbotapi.BotAPI {
	botMutex.Lock()
	defer botMutex.Unlock()

	// Если есть предыдущий клиент, останавливаем его
	if currentBot != nil {
		currentBot.StopReceivingUpdates()
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("API_KEY"))
	if err != nil {
		panic(err)
	}

	bot.Debug = debug
	log.Printf("Successfully authorized on account @%s", bot.Self.UserName)

	currentBot = bot
	return bot
}

func InActionList(update tgbotapi.Update, allowedActions []string) bool {
	if update.CallbackQuery == nil {
		return false
	}

	var data map[string]any
	err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data)

	return err == nil && slices.Contains(allowedActions, data["a"].(string))
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
		return InActionList(update, []string{"n", "p", "c"})
	}

	addSecretCallQuery := func(update tgbotapi.Update) bool {
		return InActionList(update, []string{"a"})
	}

	viewSecretCallQuery := func(update tgbotapi.Update) bool {
		return InActionList(update, []string{"s"})
	}

	act := handlers.ActiveHandlers{Handlers: []handlers.Handler{
		handlers.CommandHandler.Product(actions.MainPage{Name: "main-page-cmd", Client: *bot}, []handlers.Filter{startFilter, adminFilter}),
		handlers.CallbackQueryHandler.Product(actions.MainPage{Name: "main-page-call-query", Client: *bot}, []handlers.Filter{mainPageCallQuery, adminFilter}),
		handlers.CallbackQueryHandler.Product(actions.AddSecret{Name: "add-secret-call-query", Client: *bot}, []handlers.Filter{addSecretCallQuery, adminFilter}),
		handlers.CallbackQueryHandler.Product(actions.ViewSecret{Name: "view-secret-call-query", Client: *bot}, []handlers.Filter{viewSecretCallQuery, adminFilter}),
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
		controllers.RunStepUpdates(update, stepManager, *client)
		_ = act.HandleAll(update)
	}
}
