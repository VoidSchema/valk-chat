package cleanup

import (
	"log"
	"time"

	"valk-chat-backend/database"
	"valk-chat-backend/models"
)

func StartCleanupWorker() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		// Run immediately on start
		cleanOldMessages()
		for range ticker.C {
			cleanOldMessages()
		}
	}()
	log.Println("Message cleanup worker started (runs every 1 hour)")
}

func cleanOldMessages() {
	cutoff := time.Now().Add(-24 * time.Hour)
	result := database.DB.Unscoped().Where("created_at < ?", cutoff).Delete(&models.Message{})
	if result.Error != nil {
		log.Println("Error cleaning up old messages:", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d messages older than 24 hours\n", result.RowsAffected)
	}
}
