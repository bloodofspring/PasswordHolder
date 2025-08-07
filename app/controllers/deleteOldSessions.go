package controllers

import (
	"main/database"
	"main/database/models"
)

func DeleteOldSessions() error {
	var oldSessions []*models.Sessions
	err := database.GetDB().Model(&oldSessions).
		Where("updated_at + reset_time_interval < extract(epoch from now())").
		Select()
	if err != nil {
		return err
	}

	for _, session := range oldSessions {
		_, err := database.GetDB().Model(session).Delete()
		if err != nil {
			return err
		}
	}

	return nil
}
