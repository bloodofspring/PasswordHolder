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
	Func          NextStepFunc
	Params        map[string]any
	CreatedAtTS   int64
	CancelMessage string
	IsLastStep    bool // Флаг, указывающий, является ли этот шаг последним в цепочке
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
	log.Printf("RegisterNextStepAction: Registering new action for ChatID=%d, UserID=%d\n", stepKey.ChatID, stepKey.UserID)
	log.Printf("RegisterNextStepAction: Action details: CreatedAtTS=%d, CancelMessage=%s\n", action.CreatedAtTS, action.CancelMessage)
	n.nextStepActions[stepKey] = action
	log.Printf("RegisterNextStepAction: Current actions map after registration: %+v\n", n.nextStepActions)
}

func (n *NextStepManager) RemoveNextStepAction(stepKey NextStepKey, bot tgbotapi.BotAPI, sendCancelMessage bool) {
	log.Printf("RemoveNextStepAction: Removing action for ChatID=%d, UserID=%d\n", stepKey.ChatID, stepKey.UserID)
	log.Printf("RemoveNextStepAction: Current actions before removal: %+v\n", n.nextStepActions)

	if sendCancelMessage && n.nextStepActions[stepKey].CancelMessage != "" {
		log.Printf("RemoveNextStepAction: Sending cancel message: %s\n", n.nextStepActions[stepKey].CancelMessage)
		bot.Send(tgbotapi.NewMessage(stepKey.ChatID, n.nextStepActions[stepKey].CancelMessage))
	}

	delete(n.nextStepActions, stepKey)
	log.Printf("RemoveNextStepAction: Actions after removal: %+v\n", n.nextStepActions)
}

func (n *NextStepManager) RunUpdates(update tgbotapi.Update, client tgbotapi.BotAPI) error {
	if update.Message == nil {
		log.Println("RunUpdates: Message is nil")
		return nil
	}

	key := NextStepKey{ChatID: update.Message.Chat.ID, UserID: update.Message.From.ID}
	log.Printf("RunUpdates: Processing key ChatID=%d, UserID=%d\n", key.ChatID, key.UserID)
	log.Printf("RunUpdates: Current actions map: %+v\n", n.nextStepActions)
	action, ok := n.nextStepActions[key]

	if !ok {
		return nil
	}

	if update.Message.IsCommand() {
		return ErrMessageIsCommand
	}

	err := action.Func(client, update, action.Params)

	// Удаляем шаг только если он последний в цепочке
	if action.IsLastStep {
		GlobalNextStepManager.RemoveNextStepAction(key, client, false)
	}

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
		log.Println("error running step updates: ", err)
	}
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
