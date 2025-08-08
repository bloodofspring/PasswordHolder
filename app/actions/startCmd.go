package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"main/controllers"
	"main/crypto"
	"main/database"
	"main/database/models"
	"main/util"
	"maps"
	"math"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MainPage struct {
	Name   string
	Client tgbotapi.BotAPI
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
	var userDb models.Users
	err := database.GetDB().Model(&userDb).Where("telegram_id = ?", stepUpdate.Message.From.ID).Select()
	if err != nil {
		response := tgbotapi.NewMessage(stepUpdate.Message.Chat.ID, "Тебе тут не место.\n\nGo away.")
		_, err = client.Send(response)
		if err != nil {
			return err
		}

		return nil
	}

	if stepUpdate.Message.ReplyToMessage != nil {
		reply := stepUpdate.Message.ReplyToMessage
		stepParams["password"] = reply.Text
	} else {
		stepParams["password"] = stepUpdate.Message.Text
	}

	if crypto.HashString(stepParams["password"].(string)) != userDb.PasswordHash {
		response := tgbotapi.NewMessage(stepUpdate.Message.Chat.ID, "Неверный пароль.\n\nGo away.")
		_, err = client.Send(response)
		if err != nil {
			return err
		}

		return nil
	}

	sessionKey := crypto.GenerateRandomString(8)
	encryptedPassword, err := crypto.Encrypt(stepParams["password"].(string), sessionKey)
	if err != nil {
		return err
	}

	newSession := &models.Sessions{
		UserID:    stepUpdate.Message.From.ID,
		EncryptedPassword: encryptedPassword,
	}

	_, err = database.GetDB().Model(newSession).Insert()
	if err != nil {
		return err
	}

	return MainPage{Name: "main-page-from-step-func", Client: client}.MainPage(stepUpdate, newSession, sessionKey, false)
}

func updateSession(session *models.Sessions) error {
	session.UpdatedAt = time.Now().Unix()
	_, err := database.GetDB().Model(session).WherePK().Update()
	if err != nil {
		return err
	}

	return nil
}

func getCallbackParams(update tgbotapi.Update, offest *int, sessionKey *string, updateFromID *int64) (error) {
	var data map[string]any
	err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data)
	if err != nil {
		return err
	}

	*sessionKey = data["session_key"].(string)
	*offest = data["offest"].(int)
	*updateFromID = update.CallbackQuery.From.ID

	switch data["action"].(string) {
	case "next":
		*offest += 3
	case "prev":
		*offest -= 3
	case "add":
		log.Println("add") // TODO: Implement in other action
	case "secret":
		log.Println("secret") // TODO: Implement in other action
	}

	return nil
}

func getPageNoAndCount(offest int, updateFromID int64) (int, int, error) {
	var pageNo, pageCount int
	secretsCount, err := database.GetDB().Model(&models.Secrets{}).Where("user_id = ?", updateFromID).Count()
	if err != nil {
		return 0, 0, err
	}

	pageCount = int(math.Ceil(float64(secretsCount) / 3))
	if pageCount == 0 {
		pageNo = 0
	} else {
		pageNo = int(math.Floor(float64(offest) / 3)) + 1
	}

	return pageNo, pageCount, nil
}

func getPageText(pageNo, pageCount int) string {
	return fmt.Sprintf("Менеджер паролей Крови Весны\nСтраница: %d // %d\n\nВыберите сервис для просмотра пароля:", pageNo, pageCount)
}

func getKeyboard(pageNo, pageCount, offest int, updateFromID int64, sessionKey string) (tgbotapi.InlineKeyboardMarkup, error) {
	if pageCount != 0 {
		offest = offest % pageCount
	}

	secrets := []*models.Secrets{}
	err := database.GetDB().Model(&secrets).Where("user_id = ?", updateFromID).Offset(offest).Limit(3).Select()
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	for _, secret := range secrets {
		data := map[string]any{
			"action": "secret",
			"secret_id": secret.ID,
			"session_key": sessionKey,
			"offest": offest,
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			log.Println(err)
			continue
		}

		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(secret.Title, string(jsonData)),
		})
	}

	baseData := map[string]any{
		"session_key": sessionKey,
		"offest": offest,
	}

	var (
		nextData = map[string]any{"action": "next"}
		addData = map[string]any{"action": "add"}
		prevData = map[string]any{"action": "prev"}
	)

	maps.Copy(nextData, baseData)
	maps.Copy(addData, baseData)
	maps.Copy(prevData, baseData)

	nextDataJSON, err := json.Marshal(nextData)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	addDataJSON, err := json.Marshal(addData)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	prevDataJSON, err := json.Marshal(prevData)
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	navigationBarRow := []tgbotapi.InlineKeyboardButton{}
	
	if pageNo > 1 {
		navigationBarRow = append(navigationBarRow, tgbotapi.InlineKeyboardButton{Text: "Назад", CallbackData: util.StringPtr(string(prevDataJSON))})
	}

	navigationBarRow = append(navigationBarRow, tgbotapi.InlineKeyboardButton{Text: "+", CallbackData: util.StringPtr(string(addDataJSON))})

	if pageNo > 1 {
		navigationBarRow = append(navigationBarRow, tgbotapi.InlineKeyboardButton{Text: "Вперед", CallbackData: util.StringPtr(string(nextDataJSON))})
	}

	keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, [][]tgbotapi.InlineKeyboardButton{navigationBarRow}...)

	return keyboard, nil
}

func (m MainPage) MainPage(update tgbotapi.Update, session *models.Sessions, newSessionKey string, isCallback bool) error {
	updateSession(session)

	var offest int
	var sessionKey string
	var updateFromID int64

	if isCallback {
		err := getCallbackParams(update, &offest, &sessionKey, &updateFromID)
		if err != nil {
			return err
		}
	} else {
		offest = 0
		sessionKey = newSessionKey
		updateFromID = update.Message.From.ID
	}

	pageNo, pageCount, err := getPageNoAndCount(offest, updateFromID)
	if err != nil {
		return err
	}
	text := getPageText(pageNo, pageCount)

	keyboard, err := getKeyboard(pageNo, pageCount, offest, updateFromID, sessionKey)
	if err != nil {
		return err
	}

	response := tgbotapi.NewMessage(update.Message.Chat.ID, text)
	response.ReplyMarkup = keyboard
	_, err = m.Client.Send(response)
	if err != nil {
		return err
	}

	return nil
}

func (m MainPage) main(update tgbotapi.Update) error {
	if update.CallbackQuery != nil {
		session := &models.Sessions{}
		err := database.GetDB().Model(session).Where("user_id = ?", update.CallbackQuery.From.ID).Select()

		if err != nil {
			m.Client.Send(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
			
			return nil
		}

		return m.MainPage(update, session, "", true)
	} else if update.Message != nil {
		return m.AskPassword(update)
	}

	return nil
}

func (m MainPage) Run(update tgbotapi.Update) error {
	err := m.main(update)

	return err
}

func (e MainPage) GetName() string {
	return e.Name
}
