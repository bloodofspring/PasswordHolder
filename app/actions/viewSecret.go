package actions

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type ViewSecret struct {
	Name   string
	Client tgbotapi.BotAPI
}

func (v ViewSecret) Run(update tgbotapi.Update) error {
	return nil
}

func (v ViewSecret) GetName() string {
	return v.Name
}
