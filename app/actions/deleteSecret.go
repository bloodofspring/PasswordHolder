package actions

import (
	"encoding/json"
	"errors"
	"fmt"
	"main/database"
	"main/database/models"

	"github.com/go-pg/pg/v10"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DeleteSecret struct {
	Name   string
	Client tgbotapi.BotAPI
	DB     *pg.DB
}

func (d DeleteSecret) Run(update tgbotapi.Update) error {
	d.DB = database.GetDB()

	if update.CallbackQuery == nil {
		return errors.New("callback query is nil")
	}

	var data viewSecretCallbackData
	if err := json.Unmarshal([]byte(update.CallbackQuery.Data), &data); err != nil {
		return fmt.Errorf("failed to unmarshal callback data: %w", err)
	}

	_, err := d.DB.Model(&models.Secrets{}).Where("id = ?", data.SecretID).Delete()
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	d.Client.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Секрет удален"))

	var session models.Sessions
	err = d.DB.Model(&session).Where("user_id = ?", update.CallbackQuery.From.ID).Order("created_at DESC").Limit(1).Select()
	if err != nil {
		d.Client.Request(tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID))
		
		return nil
	}

	return MainPage{Name: "main-page-from-delete-page", Client: d.Client}.MainPage(update, &session, data.SessionKey, true)
}

func (d DeleteSecret) GetName() string {
	return d.Name
}