package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"main/crypto"
	"main/database"
	"main/database/models"
	"main/util"
	"unicode/utf16"

	"github.com/go-pg/pg/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type ViewSecret struct {
	Name   string
	Client tgbotapi.BotAPI
	DB     *pg.DB
}

type viewSecretCallbackData struct {
	Action     string `json:"a"`
	SessionKey string `json:"k"`
	Offset     int    `json:"o"`
	SecretID   int    `json:"i"`
}

func (v ViewSecret) decryptSecret(secret *models.Secrets, sessionPassword string) error {
	decryptedLogin, err := crypto.Decrypt(secret.Login, sessionPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt login: %w", err)
	}
	secret.Login = decryptedLogin

	decryptedPassword, err := crypto.Decrypt(secret.Password, sessionPassword)
	if err != nil {
		return fmt.Errorf("failed to decrypt password: %w", err)
	}
	secret.Password = decryptedPassword

	return nil
}

type keywordObj struct {
	Keyword string
	EntityName string
}

func EntityMachine(text string, keywords []keywordObj) []tgbotapi.MessageEntity {
	entities := []tgbotapi.MessageEntity{}
	textRunes := []rune(text)

	for _, keyword := range keywords {
		// Convert text and keyword to UTF-16 for proper offset calculation
		textUTF16 := utf16.Encode(textRunes)
		keywordUTF16 := utf16.Encode([]rune(keyword.Keyword))

		// Find keyword in UTF-16 encoded text
		offset := -1
		for i := 0; i <= len(textUTF16)-len(keywordUTF16); i++ {
			match := true
			for j := range keywordUTF16 {
				if textUTF16[i+j] != keywordUTF16[j] {
					match = false
					break
				}
			}
			if match {
				offset = i
				break
			}
		}

		if offset == -1 {
			continue
		}

		entities = append(entities, tgbotapi.MessageEntity{
			Type:   keyword.EntityName,
			Offset: offset,
			Length: len(keywordUTF16), // Use UTF-16 length for Telegram API
		})
	}

	return entities
}

func (v ViewSecret) formatSecretMessage(secret *models.Secrets) (string, []tgbotapi.MessageEntity) {
	messageText := fmt.Sprintf("=== %s ===\n\nЛогин: %s\nПароль: %s\nГде использовать: %s\n\nОписание:\n%s",
		secret.Title,
		secret.Login,
		secret.Password,
		secret.SiteLink,
		secret.Description,
	)

	entities := EntityMachine(messageText, []keywordObj{
		{Keyword: fmt.Sprintf("=== %s ===", secret.Title), EntityName: "bold"},
		{Keyword: secret.Login, EntityName: "code"},
		{Keyword: secret.Password, EntityName: "code"},
		{Keyword: "Описание:", EntityName: "italic"},
		{Keyword: secret.Description, EntityName: "blockquote"},
	})

	return messageText, entities
}

func (v ViewSecret) createKeyboard(data viewSecretCallbackData) tgbotapi.InlineKeyboardMarkup {
	backData := viewSecretCallbackData{
		Action:     "c",
		SessionKey: data.SessionKey,
		Offset:     data.Offset,
	}
	backDataJSON, _ := json.Marshal(backData)

	deleteData := viewSecretCallbackData{
		Action:     "d",
		SessionKey: data.SessionKey,
		Offset:     data.Offset,
		SecretID:   data.SecretID,
	}
	deleteDataJSON, _ := json.Marshal(deleteData)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
			{
				{
					Text:         "Назад",
					CallbackData: util.StringPtr(string(backDataJSON)),
				},
				{
					Text:         "Удалить",
					CallbackData: util.StringPtr(string(deleteDataJSON)),
				},
			},
		},
	}
}

func (v ViewSecret) Run(update tgbotapi.Update) error {
	v.DB = database.GetDB()

	if update.CallbackQuery == nil {
		return errors.New("callback query is nil")
	}

	var data viewSecretCallbackData
	if err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data); err != nil {
		return fmt.Errorf("failed to unmarshal callback data: %w", err)
	}

	// Проверяем наличие активной сессии
	var session models.Sessions
	err := v.DB.Model(&session).
		Where("user_id = ?", update.CallbackQuery.From.ID).
		Order("created_at DESC").
		Limit(1).
		Select()

	if err == pg.ErrNoRows {
		// Если нет активной сессии, удаляем сообщение
		deleteMsg := tgbotapi.NewDeleteMessage(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
		)
		_, err = v.Client.Send(deleteMsg)
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Расшифровываем пароль сессии
	sessionPassword, err := crypto.Decrypt(session.EncryptedPassword, data.SessionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt session password: %w", err)
	}

	// Получаем секрет
	var secret models.Secrets
	err = v.DB.Model(&secret).
		Where("id = ?", data.SecretID).
		Select()

	if err != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Расшифровываем данные секрета
	if err = v.decryptSecret(&secret, sessionPassword); err != nil {
		return err
	}

	// Форматируем сообщение и получаем entities
	messageText, entities := v.formatSecretMessage(&secret)

	// Создаем клавиатуру
	keyboard := v.createKeyboard(data)

	// Обновляем сообщение
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		update.CallbackQuery.Message.Chat.ID,
		update.CallbackQuery.Message.MessageID,
		messageText,
		keyboard,
	)
	editMsg.Entities = entities

	_, err = v.Client.Send(editMsg)
	return err
}

func (v ViewSecret) GetName() string {
	return v.Name
}
