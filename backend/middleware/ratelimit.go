package middleware

import (
	"fmt"
	"net/http"

	"valk-chat-backend/cache"

	"github.com/gin-gonic/gin"
)

func RateLimitRegister() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := cache.GetDailyCount("register")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rate limit check failed"})
			c.Abort()
			return
		}
		if count >= 10 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Batas registrasi harian tercapai (10/hari)",
				"type":  "rate_limit",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RateLimitChatGlobal() gin.HandlerFunc {
	return func(c *gin.Context) {
		count, err := cache.GetDailyCount("chat:global")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rate limit check failed"})
			c.Abort()
			return
		}
		if count >= 1000 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Batas chat global harian tercapai (1000/hari)",
				"type":  "rate_limit",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CheckUserChatLimit checks if a user has exceeded their daily chat limit (100/day)
func CheckUserChatLimit(userID uint) (bool, int64, error) {
	key := fmt.Sprintf("chat:user:%d", userID)
	count, err := cache.GetDailyCount(key)
	if err != nil {
		return false, 0, err
	}
	remaining := int64(100) - count
	if remaining < 0 {
		remaining = 0
	}
	return count < 100, remaining, nil
}

// IncrementUserChat increments both global and per-user chat counters
func IncrementUserChat(userID uint) error {
	_, err := cache.IncrementDailyCounter("chat:global")
	if err != nil {
		return err
	}
	key := fmt.Sprintf("chat:user:%d", userID)
	_, err = cache.IncrementDailyCounter(key)
	return err
}

// CheckGlobalChatLimit checks if the global chat limit has been reached (1000/day)
func CheckGlobalChatLimit() (bool, error) {
	count, err := cache.GetDailyCount("chat:global")
	if err != nil {
		return false, err
	}
	return count < 1000, nil
}

// GetUserChatRemaining returns remaining chat quota for a user
func GetUserChatRemaining(userID uint) (int64, error) {
	key := fmt.Sprintf("chat:user:%d", userID)
	count, err := cache.GetDailyCount(key)
	if err != nil {
		return 0, err
	}
	remaining := int64(100) - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}
