package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"valk-chat-backend/cache"
	"valk-chat-backend/cleanup"
	"valk-chat-backend/database"
	"valk-chat-backend/middleware"
	"valk-chat-backend/models"
	"valk-chat-backend/websocket"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Initialize Database
	database.InitDB()

	// Initialize Redis
	cache.InitRedis()

	// Start message cleanup worker (deletes messages older than 24h)
	cleanup.StartCleanupWorker()

	// Initialize WebSocket Hub
	hub := websocket.NewHub()
	go hub.Run()

	// Setup Gin Router
	r := gin.Default()

	// CORS
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	originsList := []string{"http://localhost:3000", "http://localhost:5173"}
	if allowedOrigins != "" {
		originsList = append(originsList, strings.Split(allowedOrigins, ",")...)
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     originsList,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// ===== Public Routes =====

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Register
	r.POST("/register", middleware.RateLimitRegister(), func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required,min=2,max=20"`
			Password string `json:"password" binding:"required,min=4,max=64"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username (2-20 karakter) dan password (min 4 karakter) wajib diisi"})
			return
		}

		// Check if username already exists
		var existingUser models.User
		if err := database.DB.Where("username = ?", input.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username sudah dipakai"})
			return
		}

		// Create user
		user := models.User{Username: input.Username}
		if err := user.SetPassword(input.Password); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat akun"})
			return
		}

		if err := database.DB.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat akun"})
			return
		}

		// Increment register counter
		cache.IncrementDailyCounter("register")

		// Create session
		token, err := middleware.GenerateSessionToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat session"})
			return
		}

		cache.SetSession(token, user.ID, user.Username, 24*time.Hour)
		c.SetCookie("session_token", token, 86400, "/", "", false, true)

		c.JSON(http.StatusOK, gin.H{
			"user_id":        user.ID,
			"username":       user.Username,
			"chat_remaining": 100,
		})
	})

	// Login
	r.POST("/login", func(c *gin.Context) {
		var input struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username dan password wajib diisi"})
			return
		}

		var user models.User
		if err := database.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Username atau password salah"})
			return
		}

		if !user.CheckPassword(input.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Username atau password salah"})
			return
		}

		// Create session
		token, err := middleware.GenerateSessionToken()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal membuat session"})
			return
		}

		cache.SetSession(token, user.ID, user.Username, 24*time.Hour)
		c.SetCookie("session_token", token, 86400, "/", "", false, true)

		// Get remaining chat quota
		remaining, _ := middleware.GetUserChatRemaining(user.ID)

		c.JSON(http.StatusOK, gin.H{
			"user_id":        user.ID,
			"username":       user.Username,
			"chat_remaining": remaining,
		})
	})

	// ===== Authenticated Routes =====

	// Logout
	r.POST("/logout", func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err == nil && token != "" {
			cache.DeleteSession(token)
		}
		c.SetCookie("session_token", "", -1, "/", "", false, true)
		c.JSON(http.StatusOK, gin.H{"message": "logged out"})
	})

	// Check session
	r.GET("/me", middleware.AuthRequired(), func(c *gin.Context) {
		userID := c.GetUint("user_id")
		username, _ := c.Get("username")

		remaining, _ := middleware.GetUserChatRemaining(userID)

		c.JSON(http.StatusOK, gin.H{
			"user_id":        userID,
			"username":       username,
			"chat_remaining": remaining,
		})
	})

	// Search users for mentions
	r.GET("/users/search", middleware.AuthRequired(), func(c *gin.Context) {
		query := c.Query("q")
		
		var users []models.User
		if err := database.DB.Where("username LIKE ?", query+"%").Limit(5).Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		var usernames []string
		for _, u := range users {
			usernames = append(usernames, u.Username)
		}
		c.JSON(http.StatusOK, usernames)
	})

	// Get messages (authenticated)
	r.GET("/messages", middleware.AuthRequired(), func(c *gin.Context) {
		var messages []models.Message
		// Only get messages from last 24 hours
		cutoff := time.Now().Add(-24 * time.Hour)
		if err := database.DB.Where("created_at > ?", cutoff).Order("created_at desc").Limit(50).Find(&messages).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Reverse to show in chronological order
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}

		c.JSON(http.StatusOK, messages)
	})

	// WebSocket (authenticated via cookie)
	r.GET("/ws", func(c *gin.Context) {
		token, err := c.Cookie("session_token")
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID, username, err := cache.GetSession(token)
		if err != nil || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}
		websocket.ServeWs(hub, c.Writer, c.Request, userID, username)
	})

	// Start Server
	log.Println("Server started on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
