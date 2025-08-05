package main

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

func GetMessage(update tgbotapi.Update) *tgbotapi.Message {
	switch {
	case update.Message != nil:
		return update.Message
	case update.CallbackQuery != nil:
		return update.CallbackQuery.Message
	default:
		return nil
	}
}
