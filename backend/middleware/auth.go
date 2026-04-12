package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"valk-chat-backend/cache"

	"github.com/gin-gonic/gin"
)

func GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		userID, username, err := cache.GetSession(token)
		if err != nil || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Set("username", username)
		c.Set("session_token", token)
		c.Next()
	}
}
