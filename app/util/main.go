package util

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

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
