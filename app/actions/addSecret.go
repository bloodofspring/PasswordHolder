package actions

import (
	"encoding/json"
	"log"

	// "log"
	"main/controllers"
	"main/crypto"
	"main/database"
	"main/database/models"
	"main/util"
	"maps"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type AddSecret struct {
	Name   string
	Client tgbotapi.BotAPI
}

// baseForm отображает форму ввода с кнопкой отмены и регистрирует следующий шаг.
func baseForm(client tgbotapi.BotAPI, update tgbotapi.Update, params map[string]any, formText, CancelMessage string, formHandler controllers.NextStepFunc, cancelCallbackData string, isLastStep bool) error {
	client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(update).Chat.ID, util.GetMessage(update).MessageID-1))
	client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(update).Chat.ID, util.GetMessage(update).MessageID))

	msg := tgbotapi.NewMessage(util.GetMessage(update).Chat.ID, formText)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Отмена", cancelCallbackData),
		),
	)
	_, err := client.Send(msg)
	if err != nil {
		return err
	}

	stepKey := controllers.NextStepKey{
		UserID: util.GetMessage(update).From.ID,
		ChatID: util.GetMessage(update).Chat.ID,
	}

	stepAction := controllers.NextStepAction{
		Func:          formHandler,
		Params:        params,
		CreatedAtTS:   time.Now().Unix(),
		CancelMessage: CancelMessage,
		IsLastStep:    isLastStep,
	}

	controllers.GetNextStepManager().RegisterNextStepAction(stepKey, stepAction)

	return nil
}

func (a AddSecret) StartPoll(update tgbotapi.Update) error {
	callbackDataParams := make(map[string]any)
	err := json.Unmarshal([]byte(update.CallbackQuery.Data), &callbackDataParams)
	if err != nil {
		return err
	}

	stepParams := make(map[string]any)

	stepParams["session_key"] = callbackDataParams["k"]
	stepParams["page_offest"] = callbackDataParams["o"]
	stepParams["client"] = a.Client
	stepParams["update"] = update

	cancelParams := make(map[string]any)
	maps.Copy(cancelParams, callbackDataParams)
	cancelParams["a"] = "c"

	cancelParamsJSON, err := json.Marshal(cancelParams)
	if err != nil {
		return err
	}

	stepParams["on_cancel"] = string(cancelParamsJSON)

	return baseForm(
		stepParams["client"].(tgbotapi.BotAPI),
		stepParams["update"].(tgbotapi.Update),
		stepParams,
		"Отправьте название секрета ниже:",
		"Создание секрета отменено",
		getTitle,
		stepParams["on_cancel"].(string),
		false,
	)
}

func finishPollWithoutSession(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update) bool {
	if util.HasActiveSession(stepUpdate) {
		return false
	}

	log.Println("deleting messages...")

	client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID-1))
	client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID))

	return true
}

func encryptDataWithSessionPassword(stepParams map[string]any, data string) (string, error) {
	session, err := util.GetSession(stepParams["update"].(tgbotapi.Update))
	if err != nil {
		log.Printf("Failed to get session: %v", err)
		return "", err
	}

	sessionKey, ok := stepParams["session_key"].(string)
	if !ok {
		log.Printf("Invalid session key type: %T", stepParams["session_key"])
		return "", err
	}

	if session.EncryptedPassword == "" {
		log.Printf("Empty encrypted password in session")
		return "", err
	}

	decryptedPassword, err := crypto.Decrypt(session.EncryptedPassword, sessionKey)
	if err != nil {
		log.Printf("Failed to decrypt session password: %v", err)
		return "", err
	}

	encrypted, err := crypto.Encrypt(data, decryptedPassword)
	if err != nil {
		log.Printf("Failed to encrypt data: %v", err)
		return "", err
	}

	return encrypted, nil
}

func getTitle(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	secret := &models.Secrets{Title: stepUpdate.Message.Text}
	stepParams["update"] = stepUpdate
	stepParams["new_secret"] = secret

	if finishPollWithoutSession(client, stepUpdate) {
		return nil
	}

	return baseForm(
		stepParams["client"].(tgbotapi.BotAPI),
		stepParams["update"].(tgbotapi.Update),
		stepParams,
		"Отправьте ваш логин:",
		"Создание секрета отменено",
		getLogin,
		stepParams["on_cancel"].(string),
		false,
	)
}

func getLogin(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	stepParams["update"] = stepUpdate
	if finishPollWithoutSession(client, stepUpdate) {
		return nil
	}

	encryptedLogin, err := encryptDataWithSessionPassword(stepParams, stepUpdate.Message.Text)
	if err != nil {
		client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID-1))
		client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID))

		return err
	}

	editedSecret := stepParams["new_secret"].(*models.Secrets)
	editedSecret.Login = encryptedLogin
	*stepParams["new_secret"].(*models.Secrets) = *editedSecret

	return baseForm(
		stepParams["client"].(tgbotapi.BotAPI),
		stepParams["update"].(tgbotapi.Update),
		stepParams,
		"Отправьте ваш пароль:",
		"Создание секрета отменено",
		getPassword,
		stepParams["on_cancel"].(string),
		false,
	)
}

func getPassword(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	stepParams["update"] = stepUpdate
	if finishPollWithoutSession(client, stepUpdate) {
		return nil
	}

	encryptedPassword, err := encryptDataWithSessionPassword(stepParams, stepUpdate.Message.Text)
	if err != nil {
		client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID-1))
		client.Request(tgbotapi.NewDeleteMessage(util.GetMessage(stepUpdate).Chat.ID, util.GetMessage(stepUpdate).MessageID))

		return err
	}

	editedSecret := stepParams["new_secret"].(*models.Secrets)
	editedSecret.Password = encryptedPassword
	*stepParams["new_secret"].(*models.Secrets) = *editedSecret

	return baseForm(
		stepParams["client"].(tgbotapi.BotAPI),
		stepParams["update"].(tgbotapi.Update),
		stepParams,
		"Отправьте ссылку на ресурс (Или \"-\" чтобы пропустить):",
		"Создание секрета отменено",
		getSiteLink,
		stepParams["on_cancel"].(string),
		false,
	)
}

func getSiteLink(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	stepParams["update"] = stepUpdate
	if finishPollWithoutSession(client, stepUpdate) {
		return nil
	}

	if stepUpdate.Message.Text != "-" {
		editedSecret := stepParams["new_secret"].(*models.Secrets)
		editedSecret.SiteLink = stepUpdate.Message.Text
		*stepParams["new_secret"].(*models.Secrets) = *editedSecret
	}

	return baseForm(
		stepParams["client"].(tgbotapi.BotAPI),
		stepParams["update"].(tgbotapi.Update),
		stepParams,
		"Отправьте описание секрета (Или \"-\" чтобы пропустить):",
		"Создание секрета отменено",
		getDescriptionAndFinishPoll,
		stepParams["on_cancel"].(string),
		true,
	)
}

func getDescriptionAndFinishPoll(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error {
	stepParams["update"] = stepUpdate
	if finishPollWithoutSession(client, stepUpdate) {
		return nil
	}

	if stepUpdate.Message.Text != "-" {
		editedSecret := stepParams["new_secret"].(*models.Secrets)
		editedSecret.Description = stepUpdate.Message.Text
		*stepParams["new_secret"].(*models.Secrets) = *editedSecret
	}

	db := database.GetDB()
	editedSecret := stepParams["new_secret"].(*models.Secrets)
	editedSecret.UserID = util.GetMessage(stepUpdate).From.ID
	*stepParams["new_secret"].(*models.Secrets) = *editedSecret

	_, err := db.Model(stepParams["new_secret"].(*models.Secrets)).Insert()
	if err != nil {
		return err
	}

	client.Request(tgbotapi.NewDeleteMessage(stepUpdate.Message.Chat.ID, stepUpdate.Message.MessageID-1))
	client.Request(tgbotapi.NewDeleteMessage(stepUpdate.Message.Chat.ID, stepUpdate.Message.MessageID))

	callbackData := map[string]any{
		"k": stepParams["session_key"],
		"o": stepParams["page_offest"],
		"a": "c",
	}
	callbackDataJSON, err := json.Marshal(callbackData)
	if err != nil {
		return err
	}

	response := tgbotapi.NewMessage(stepUpdate.Message.Chat.ID, "Секрет успешно создан!")
	response.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{
			{Text: "К секретам", CallbackData: util.StringPtr(string(callbackDataJSON))},
		}},
	}

	_, err = client.Request(response)

	return err
}

func (a AddSecret) Run(update tgbotapi.Update) error {
	controllers.ClearNextStepForUser(update, &a.Client, true)

	return a.StartPoll(update)
}

func (a AddSecret) GetName() string {
	return a.Name
}
