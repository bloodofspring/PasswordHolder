package actions

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

type MainPage struct {
	Name   string
	Client tgbotapi.BotAPI
}

func (e MainPage) fabricateAnswer(update tgbotapi.Update) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	msg.Text = "Hi :)"

	return msg
}

func (e MainPage) Run(update tgbotapi.Update) error {
	if _, err := e.Client.Send(e.fabricateAnswer(update)); err != nil {
		return err
	}

	return nil
}

func (e MainPage) GetName() string {
	return e.Name
}
