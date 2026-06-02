package websocket

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"personal-finance-manager/internal/models"
)

type Client struct {
	userID int64
	conn   *websocket.Conn
	send   chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
	reg     chan *Client
	unreg   chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int64]map[*Client]struct{}),
		reg:     make(chan *Client, 64),
		unreg:   make(chan *Client, 64),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.reg:
			h.mu.Lock()
			if h.clients[c.userID] == nil {
				h.clients[c.userID] = make(map[*Client]struct{})
			}
			h.clients[c.userID][c] = struct{}{}
			h.mu.Unlock()

		case c := <-h.unreg:
			h.mu.Lock()
			if set, ok := h.clients[c.userID]; ok {
				delete(set, c)
				if len(set) == 0 {
					delete(h.clients, c.userID)
				}
			}
			h.mu.Unlock()
			close(c.send)
		}
	}
}

func (h *Hub) BroadcastToUser(userID int64, msg models.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients[userID] {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (h *Hub) register(c *Client)   { h.reg <- c }
func (h *Hub) unregister(c *Client) { h.unreg <- c }
