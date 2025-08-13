package controllers

import (
	"errors"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	StepTimeout = 3600
)

var (
	ErrMessageIsCommand = errors.New("message is command")
)

type NextStepKey struct {
	ChatID int64
	UserID int64
}

type NextStepFunc func(client tgbotapi.BotAPI, stepUpdate tgbotapi.Update, stepParams map[string]any) error

type NextStepAction struct {
	Func        NextStepFunc
	Params      map[string]any
	CreatedAtTS int64
	CancelMessage string
}

type NextStepManager struct {
	nextStepActions map[NextStepKey]NextStepAction
}

// Глобальный экземпляр NextStepManager
var GlobalNextStepManager = &NextStepManager{
	nextStepActions: make(map[NextStepKey]NextStepAction),
}

// GetNextStepManager возвращает глобальный экземпляр NextStepManager
func GetNextStepManager() *NextStepManager {
	return GlobalNextStepManager
}

func (n *NextStepManager) RegisterNextStepAction(stepKey NextStepKey, action NextStepAction) {
	n.nextStepActions[stepKey] = action
}

func (n NextStepManager) RemoveNextStepAction(stepKey NextStepKey, bot tgbotapi.BotAPI, sendCancelMessage bool) {
	if sendCancelMessage && n.nextStepActions[stepKey].CancelMessage != "" {
		bot.Send(tgbotapi.NewMessage(stepKey.ChatID, n.nextStepActions[stepKey].CancelMessage))
	}

	delete(n.nextStepActions, stepKey)
}

func (n NextStepManager) RunUpdates(update tgbotapi.Update, client tgbotapi.BotAPI) error {
	if update.Message == nil {
		return nil
	}

	key := NextStepKey{ChatID: update.Message.Chat.ID, UserID: update.Message.From.ID}
	action, ok := n.nextStepActions[key]

	if !ok {
		return nil
	}

	if update.Message.IsCommand() {
		return ErrMessageIsCommand
	}

	err := action.Func(client, update, action.Params)

	GlobalNextStepManager.RemoveNextStepAction(key, client, false)

	return err
}

func (n *NextStepManager) ClearOldSteps(client tgbotapi.BotAPI) (int, error) {
	now := time.Now().Unix()
	deleted := 0

	for key, action := range n.nextStepActions {
		if now-action.CreatedAtTS > StepTimeout {
			n.RemoveNextStepAction(key, client, true)
			deleted++
		}
	}

	return deleted, nil
}

func RunStepUpdates(update tgbotapi.Update, stepManager *NextStepManager, client tgbotapi.BotAPI) {
	err := stepManager.RunUpdates(update, client)
	if err != nil {
		return
	}

	stepsCleaned, err := stepManager.ClearOldSteps(client)
	if err != nil {
		log.Println("error clearing old steps: ", err)
		return
	}

	log.Println("stepsCleaned: ", stepsCleaned)
}

// ClearNextStepForUser очищает следующий шаг для пользователя
// update - обновление от Telegram API
// client - экземпляр Telegram бота
// sendCancelMessage - флаг, указывающий, нужно ли отправлять сообщение об отмене
func ClearNextStepForUser(update tgbotapi.Update, client *tgbotapi.BotAPI, sendCancelMessage bool) {
	var user *tgbotapi.User
	var chat *tgbotapi.Chat

	switch {
	case update.Message != nil:
		user = update.Message.From
	case update.CallbackQuery != nil:
		user = update.CallbackQuery.From
	default:
		return
	}

	switch {
	case update.Message != nil:
		chat = update.Message.Chat
	case update.CallbackQuery != nil:
		chat = update.CallbackQuery.Message.Chat
	}

	GetNextStepManager().RemoveNextStepAction(NextStepKey{
		ChatID: chat.ID,
		UserID: user.ID,
	}, *client, sendCancelMessage)
}

