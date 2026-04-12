package websocket

import (
	"encoding/json"
	"log"
	"valk-chat-backend/database"
	"valk-chat-backend/models"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan models.Message

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan models.Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			// Save message to database
			if err := database.DB.Create(&message).Error; err != nil {
				log.Println("Error saving message:", err)
			}

			// Wrap message in protocol envelope
			payload, err := json.Marshal(map[string]interface{}{
				"type": "message",
				"data": message,
			})
			if err != nil {
				log.Println("Error marshaling message:", err)
				continue
			}

			for client := range h.clients {
				select {
				case client.send <- payload:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
