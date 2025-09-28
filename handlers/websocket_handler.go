package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"hcs-full/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var WsHub *Hub

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, you should validate the origin.
	},
}

// Client is a middleman between the WebSocket connection and the hub.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	UserID uuid.UUID // Will be Nil for admins
}

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	admins     map[*Client]bool
	unicast    chan *models.Message
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		unicast:    make(chan *models.Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		admins:     make(map[*Client]bool),
	}
}

// BroadcastMessage sends a message to all connected admins.
func (h *Hub) BroadcastMessage(message []byte) {
	h.broadcast <- message
}

// UnicastMessage sends a message to a specific user client.
func (h *Hub) UnicastMessage(message *models.Message) {
	h.unicast <- message
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			if client.UserID == uuid.Nil { // It's an admin
				h.admins[client] = true
			} else {
				h.clients[client] = true
			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			if _, ok := h.admins[client]; ok {
				delete(h.admins, client)
				close(client.send)
			}
		case message := <-h.broadcast: // Broadcast to all admins
			for client := range h.admins {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.admins, client)
				}
			}
		case message := <-h.unicast: // Send to a specific user
			jsonMsg, err := json.Marshal(message)
			if err != nil {
				log.Printf("error marshalling unicast message: %v", err)
				continue
			}
			for client := range h.clients {
				// Note: message.UserID from the models.Message struct is a uuid.UUID
				if client.UserID == message.UserID {
					select {
					case client.send <- jsonMsg:
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
			}
		}
	}
}

func (c *Client) writePump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

// ServeWs handles websocket requests from authenticated users.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("userClaims").(*models.Claims)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	var clientUserID uuid.UUID
	if !claims.IsAdmin {
		clientUserID = claims.UserID
	}

	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256), UserID: clientUserID}
	client.hub.register <- client

	go client.writePump()
}
