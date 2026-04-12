package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	"valk-chat-backend/middleware"
	"valk-chat-backend/models"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allowing all origins for this project
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub *Hub

	// The websocket connection.
	Conn *websocket.Conn

	// Buffered channel of outbound messages (JSON bytes).
	send chan []byte

	// Authenticated user info
	UserID   uint
	Username string
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		// Read raw JSON from client
		var incoming struct {
			Content string `json:"content"`
		}
		err := c.Conn.ReadJSON(&incoming)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		if incoming.Content == "" {
			continue
		}

		// Check global chat limit
		globalAllowed, err := middleware.CheckGlobalChatLimit()
		if err != nil {
			log.Println("Error checking global chat limit:", err)
			continue
		}
		if !globalAllowed {
			errPayload, _ := json.Marshal(map[string]interface{}{
				"type":  "rate_limit",
				"error": "Batas chat global harian tercapai (1000/hari)",
			})
			c.send <- errPayload
			break
		}

		// Check per-user chat limit
		allowed, remaining, err := middleware.CheckUserChatLimit(c.UserID)
		if err != nil {
			log.Println("Error checking user chat limit:", err)
			continue
		}
		if !allowed {
			errPayload, _ := json.Marshal(map[string]interface{}{
				"type":      "rate_limit",
				"error":     "Batas chat harian kamu tercapai (100/hari)",
				"remaining": remaining,
			})
			c.send <- errPayload
			break
		}

		// Increment rate limit counters
		if err := middleware.IncrementUserChat(c.UserID); err != nil {
			log.Println("Error incrementing chat counter:", err)
		}

		msg := models.Message{
			UserID:    c.UserID,
			Username:  c.Username,
			Content:   incoming.Content,
			CreatedAt: time.Now(),
		}
		c.Hub.broadcast <- msg
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWs handles websocket requests from the peer.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID uint, username string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{
		Hub:      hub,
		Conn:     conn,
		send:     make(chan []byte, 256),
		UserID:   userID,
		Username: username,
	}
	client.Hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
