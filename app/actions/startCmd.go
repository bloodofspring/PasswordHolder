package actions

import (
	"encoding/json"
	"fmt"
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

const (
	BUTTONS_PER_PAGE            = 6
	SESSION_RESET_TIME_INTERVAL = 1000 // 600
)

type MainPage struct {
	Name   string
	Client tgbotapi.BotAPI
}

func (m MainPage) AskPassword(update tgbotapi.Update) error {
	m.Client.Request(tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID))
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
	client.Request(tgbotapi.NewDeleteMessage(stepUpdate.Message.Chat.ID, stepUpdate.Message.MessageID-1))
	client.Request(tgbotapi.NewDeleteMessage(stepUpdate.Message.Chat.ID, stepUpdate.Message.MessageID))

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
		UserID:            stepUpdate.Message.From.ID,
		EncryptedPassword: encryptedPassword,
		ResetTimeInterval: SESSION_RESET_TIME_INTERVAL,
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

func getCallbackParams(update tgbotapi.Update, offest *int, sessionKey *string, updateFromID *int64) error {
	var data map[string]any
	err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data)
	if err != nil {
		return err
	}

	*sessionKey = data["k"].(string)
	*offest = int(data["o"].(float64))
	*updateFromID = update.CallbackQuery.From.ID

	switch data["a"].(string) {
	case "n": // next
		*offest += BUTTONS_PER_PAGE
	case "p": // prev
		*offest -= BUTTONS_PER_PAGE
	}

	return nil
}

func getPageNoAndCount(offest int, updateFromID int64) (int, int, error) {
	var pageNo, pageCount int
	secretsCount, err := database.GetDB().Model(&models.Secrets{}).Where("user_id = ?", updateFromID).Count()
	if err != nil {
		return 0, 0, err
	}

	pageCount = int(math.Ceil(float64(secretsCount) / float64(BUTTONS_PER_PAGE)))
	if pageCount == 0 {
		pageNo = 0
	} else {
		// Ensure offset is within bounds before calculating page number
		totalItems := pageCount * BUTTONS_PER_PAGE
		if offest >= totalItems {
			offest = 0
		} else if offest < 0 {
			offest = totalItems - BUTTONS_PER_PAGE
			if offest < 0 {
				offest = 0
			}
		}
		pageNo = (offest / BUTTONS_PER_PAGE) + 1
	}

	return pageNo, pageCount, nil
}

func getPageText(pageNo, pageCount int) string {
	return fmt.Sprintf("Менеджер паролей Крови Весны\nСтраница: %d // %d\n\nВыберите сервис для просмотра пароля:", pageNo, pageCount)
}

func getKeyboard(pageCount, offest int, updateFromID int64, sessionKey string) (tgbotapi.InlineKeyboardMarkup, error) {
	totalItems := pageCount * BUTTONS_PER_PAGE

	if totalItems > 0 {
		// Ensure offset stays within bounds
		if offest >= totalItems {
			offest = 0
		} else if offest < 0 {
			offest = totalItems - BUTTONS_PER_PAGE
			if offest < 0 {
				offest = 0
			}
		}
	} else {
		offest = 0
	}

	secrets := []*models.Secrets{}
	err := database.GetDB().Model(&secrets).Where("user_id = ?", updateFromID).Offset(offest).Limit(BUTTONS_PER_PAGE).Select()
	if err != nil {
		return tgbotapi.InlineKeyboardMarkup{}, err
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup()
	for i := 0; i < len(secrets); i += 2 {
		baseData := map[string]any{
			"a": "s",        // action: secret
			"k": sessionKey, // session_key
			"o": offest,     // offest
		}

		firstSecret := map[string]any{"i": secrets[i].ID}
		maps.Copy(firstSecret, baseData)
		firstSecretJSON, err := json.Marshal(firstSecret)
		if err != nil {
			return tgbotapi.InlineKeyboardMarkup{}, err
		}

		buttonRow := []tgbotapi.InlineKeyboardButton{}
		buttonRow = append(buttonRow, tgbotapi.InlineKeyboardButton{Text: secrets[i].Title, CallbackData: util.StringPtr(string(firstSecretJSON))})

		if i+1 < len(secrets) {
			secondSecret := map[string]any{"i": secrets[i+1].ID}
			maps.Copy(secondSecret, baseData)
			secondSecretJSON, err := json.Marshal(secondSecret)
			if err != nil {
				return tgbotapi.InlineKeyboardMarkup{}, err
			}

			buttonRow = append(buttonRow, tgbotapi.InlineKeyboardButton{Text: secrets[i+1].Title, CallbackData: util.StringPtr(string(secondSecretJSON))})
		}

		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, buttonRow)
	}

	baseData := map[string]any{
		"k": sessionKey,
		"o": offest,
	}

	var (
		nextData = map[string]any{"a": "n"} // action: next
		addData  = map[string]any{"a": "a"} // action: add
		prevData = map[string]any{"a": "p"} // action: prev
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

	if pageCount > 1 {
		navigationBarRow = append(navigationBarRow, tgbotapi.InlineKeyboardButton{Text: "Назад", CallbackData: util.StringPtr(string(prevDataJSON))})
	}

	navigationBarRow = append(navigationBarRow, tgbotapi.InlineKeyboardButton{Text: "+", CallbackData: util.StringPtr(string(addDataJSON))})

	if pageCount > 1 {
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

	keyboard, err := getKeyboard(pageCount, offest, updateFromID, sessionKey)
	if err != nil {
		return err
	}

	if !isCallback {
		response := tgbotapi.NewMessage(updateFromID, text)
		response.ReplyMarkup = keyboard

		_, err = m.Client.Send(response)
	} else {
		response := tgbotapi.NewEditMessageText(updateFromID, update.CallbackQuery.Message.MessageID, text)
		response.ReplyMarkup = &keyboard

		_, err = m.Client.Request(response)
	}

	return err
}

func (m MainPage) main(update tgbotapi.Update) error {
	if update.CallbackQuery != nil {
		session, err := util.GetSession(update)

		if err != nil {
			m.Client.Send(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))

			return nil
		}

		return m.MainPage(update, &session, "", true)
	} else if update.Message != nil {
		database.GetDB().Model(&models.Sessions{}).Where("user_id = ?", update.Message.From.ID).Delete()

		return m.AskPassword(update)
	}

	return nil
}

func (m MainPage) Run(update tgbotapi.Update) error {
	err := m.main(update)

	return err
}

func (m MainPage) GetName() string {
	return m.Name
}
