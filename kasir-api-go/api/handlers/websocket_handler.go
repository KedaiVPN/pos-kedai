package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"kasir-api-go/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now (MVP)
	},
}

type WebSocketManager struct {
	clients map[string]map[*websocket.Conn]bool
	mu      sync.Mutex
}

func NewWebSocketManager() *WebSocketManager {
	return &WebSocketManager{
		clients: make(map[string]map[*websocket.Conn]bool),
	}
}

func (wm *WebSocketManager) HandleConnections(c *gin.Context) {
	storeID := c.Query("store_id")
	// If no store_id is provided, use a default "admin" group for backward compatibility
	if storeID == "" {
		storeID = "admin"
	}

	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	wm.mu.Lock()
	if wm.clients[storeID] == nil {
		wm.clients[storeID] = make(map[*websocket.Conn]bool)
	}
	wm.clients[storeID][ws] = true
	wm.mu.Unlock()

	log.Println("Client connected to WebSocket for store:", storeID)

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("Client disconnected:", err)
			wm.mu.Lock()
			delete(wm.clients[storeID], ws)
			if len(wm.clients[storeID]) == 0 {
				delete(wm.clients, storeID)
			}
			wm.mu.Unlock()
			break
		}
	}
}

type ProductNotification struct {
	Type    string            `json:"type"`
	Product db.PendingProduct `json:"product"`
}

func (wm *WebSocketManager) BroadcastNewPendingProduct(product db.PendingProduct) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	notification := ProductNotification{
		Type:    "NEW_PENDING_PRODUCT",
		Product: product,
	}

	message, err := json.Marshal(notification)
	if err != nil {
		log.Println("Failed to marshal notification:", err)
		return
	}

	if adminClients, ok := wm.clients["admin"]; ok {
		for client := range adminClients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("WebSocket write error:", err)
				client.Close()
				delete(wm.clients["admin"], client)
			}
		}
	}
}

func (wm *WebSocketManager) BroadcastSyncProduct(storeID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	notification := map[string]string{
		"type": "SYNC_PRODUCT",
	}

	message, err := json.Marshal(notification)
	if err != nil {
		log.Println("Failed to marshal notification:", err)
		return
	}

	if clients, ok := wm.clients[storeID]; ok {
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("WebSocket write error:", err)
				client.Close()
				delete(wm.clients[storeID], client)
			}
		}
	}
}
