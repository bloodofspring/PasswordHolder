package actions

import (
	"main/controllers"
	"main/database"
	"main/database/models"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MainPage struct {
	Name   string
	Client tgbotapi.BotAPI
}

func (m MainPage) GetSession(update tgbotapi.Update) (*models.Sessions, error) {
	session := &models.Sessions{}
	err := database.GetDB().Model(session).Where("user_id = ?", update.Message.From.ID).Select()

	if err != nil {
		return nil, err
	}

	return session, nil
}

func (m MainPage) AskPassword(update tgbotapi.Update) error {
	response := tgbotapi.NewMessage(update.Message.Chat.ID, "Введите пароль или отправьте реплай на сообщение, текст которого содержит пароль:")
	_, err := m.Client.Send(response)
	if err != nil {
		return err
	}

	stepKey := controllers.NextStepKey{
		ChatID: update.Message.Chat.ID,
		UserID: update.Message.From.ID,
	}
	stepAction := controllers.NextStepAction{
		Func:        HandlePassword,
		Params:      make(map[string]any),
		CreatedAtTS: time.Now().Unix(),
	}

	controllers.GetNextStepManager().RegisterNextStepAction(stepKey, stepAction)

	return nil
}

func HandlePassword(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	if stepUpdate.Message.ReplyToMessage != nil {
		reply := stepUpdate.Message.ReplyToMessage
		stepParams["password"] = reply.Text
	} else {
		stepParams["password"] = stepUpdate.Message.Text
	}

	newSession := &models.Sessions{
		UserID:    stepUpdate.Message.From.ID,
		Password:  stepParams["password"].(string),
	}

	_, err := database.GetDB().Model(newSession).Insert()
	if err != nil {
		return err
	}

	return MainPage{Name: "main-page-from-step-func", Client: client}.MainPage(stepUpdate, newSession)
}

func (m MainPage) MainPage(update tgbotapi.Update, session *models.Sessions) error {
	response := tgbotapi.NewMessage(update.Message.Chat.ID, "Главная страница")
	_, err := m.Client.Send(response)
	if err != nil {
		return err
	}

	return nil
}

func (m MainPage) main(update tgbotapi.Update) error {
	session, err := m.GetSession(update)
	if err != nil {
		return m.AskPassword(update)
	}

	return m.MainPage(update, session)
}

func (m MainPage) Run(update tgbotapi.Update) error {
	err := m.main(update)

	return err
}

func (e MainPage) GetName() string {
	return e.Name
}
