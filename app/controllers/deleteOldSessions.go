package controllers

import (
	"main/database"
	"main/database/models"
)

func DeleteOldSessions() error {
	_, err := database.GetDB().Model(&models.Sessions{}).
		Where("updated_at + reset_time_interval < extract(epoch from now())").
		Delete()

	return err
}
