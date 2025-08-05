package handlers

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type Filter func(update tgbotapi.Update) bool

type Callback interface {
	Run(update tgbotapi.Update) error
	GetName() string
}

type Handler interface {
	checkType(update tgbotapi.Update) bool
	checkFilters(update tgbotapi.Update) bool
	run(update tgbotapi.Update) (bool, error)
	getId() uuid.UUID
}

type BaseHandler struct {
	uuid      uuid.UUID
	queryType string
	callback  Callback
	filters   []Filter
}

func (h BaseHandler) getId() uuid.UUID {
	return h.uuid
}

func (h BaseHandler) checkType(update tgbotapi.Update) bool {
	switch h.queryType {
	case "message":
		return update.Message != nil
	case "callbackQuery":
		return update.CallbackQuery != nil
	case "command":
		return update.Message != nil && update.Message.IsCommand()
	default:
		fmt.Printf("WARNING! Unsupported query type: %s\nYou can edit handlers in handlers.go file", h.queryType)
		return false
	}
}

func (h BaseHandler) checkFilters(update tgbotapi.Update) bool {
	for _, f := range h.filters {
		if !f(update) {
			return false
		}
	}

	return true
}

func (h BaseHandler) run(update tgbotapi.Update) (bool, error) {
	if h.checkType(update) && h.checkFilters(update) {
		return true, h.callback.Run(update)
	}

	return false, nil
}

type ActiveHandlers struct {
	Handlers []Handler
}

func (hl ActiveHandlers) HandleAll(update tgbotapi.Update) map[uuid.UUID]bool {
	result := make(map[uuid.UUID]bool)

	for _, h := range hl.Handlers {
		runResult, err := h.run(update)

		if err != nil {
			panic(err)
		}

		result[h.getId()] = runResult
	}

	return result
}

type handlerProducer struct {
	handlerType string
}

func (p handlerProducer) Product(callback Callback, filters []Filter) BaseHandler {
	return BaseHandler{
		uuid:      uuid.New(),
		queryType: p.handlerType,
		callback:  callback,
		filters:   filters,
	}
}

const messageType = "message"
const commandType = "command"
const callbackQueryType = "callbackQuery"

var MessageHandler = handlerProducer{messageType}
var CommandHandler = handlerProducer{commandType}
var CallbackQueryHandler = handlerProducer{callbackQueryType}
