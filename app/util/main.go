package util

import (
	"main/database"
	"main/database/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func GetMessage(update tgbotapi.Update) *tgbotapi.Message {
	switch {
	case update.Message != nil:
		return update.Message
	case update.CallbackQuery != nil:
		message := update.CallbackQuery.Message
		message.From = update.CallbackQuery.From
		
		return message
	default:
		return nil
	}
}

func StringPtr(s string) *string {
	return &s
}

func GetSession(update tgbotapi.Update) (models.Sessions, error) {
	session := &models.Sessions{}
	err := database.GetDB().Model(session).Where("user_id = ?", GetMessage(update).From.ID).Select()

	return *session, err
}

func HasActiveSession(update tgbotapi.Update) bool {
	_, err := GetSession(update)

	return err == nil
}
