package main

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Room maintains the set of active clients and broadcasts messages to the clients.
type Room struct {
	name        string
	clients     map[*Client]bool
	broadcast   chan []byte
	register    chan *Client
	unregister  chan *Client
	lastContent []byte
}

func newRoom(name string) *Room {
	return &Room{
		name:       name,
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.register:
			r.clients[client] = true
			if len(r.lastContent) > 0 {
				client.send <- r.lastContent
			}
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
			}
		case message := <-r.broadcast:
			r.lastContent = message
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(r.clients, client)
				}
			}
		}
	}
}

// RoomManager manages all the rooms
type RoomManager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func (rm *RoomManager) getRoom(name string) *Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, ok := rm.rooms[name]; ok {
		return room
	}

	room := newRoom(name)
	rm.rooms[name] = room
	go room.run()
	return room
}

var roomManager = &RoomManager{
	rooms: make(map[string]*Room),
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	room *Room
	conn *websocket.Conn
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
// We don't expect clients to send messages, but we need to read to handle close and pong.
func (c *Client) readPump() {
	defer func() {
		c.room.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	// Extract room ID from URL. Assuming /ws/{roomID}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}
	roomID := pathParts[2]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	room := roomManager.getRoom(roomID)
	client := &Client{room: room, conn: conn, send: make(chan []byte, 256)}
	client.room.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

func handlePublish(w http.ResponseWriter, r *http.Request) {
	// Serve static files for the frontend
	if r.URL.Path == "/" || r.URL.Path == "/index.html" {
		http.ServeFile(w, r, "./public/index.html")
		return
	}
	if r.URL.Path == "/style.css" {
		http.ServeFile(w, r, "./public/style.css")
		return
	}
	if r.URL.Path == "/app.js" {
		http.ServeFile(w, r, "./public/app.js")
		return
	}
	if r.URL.Path == "/qrcode.min.js" {
		http.ServeFile(w, r, "./public/qrcode.min.js")
		return
	}

	// Extract room ID from URL. Assuming /{roomID}
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 2 {
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}
	roomID := pathParts[1]

	// If the path is just "/", ignore or handle root
	if roomID == "" {
		http.Error(w, "Missing room ID", http.StatusBadRequest)
		return
	}

	content := r.URL.Query().Get("content")
	if content == "" {
		http.Error(w, "Missing content parameter", http.StatusBadRequest)
		return
	}

	room := roomManager.getRoom(roomID)
	room.broadcast <- []byte(content)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Published to " + roomID))
}

func main() {
	// Subscriber endpoint: /ws/{roomID}
	http.HandleFunc("/ws/", serveWs)

	// Publisher endpoint: /{roomID}?content=...
	// We use a catch-all pattern or specific handler.
	// Since http.HandleFunc matches prefixes, "/" will match everything not matched by others.
	// But we need to be careful not to capture /ws/ if we defined it.
	// The specific pattern "/ws/" takes precedence over "/".
	http.HandleFunc("/", handlePublish)

	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
