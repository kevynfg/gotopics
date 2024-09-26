package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kevynfg/gotopics/utils"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

type CreatedRoomMessage struct {
	Type string `json:"type"`
	Data struct {
		Host   string `json:"host"`
		RoomID string `json:"roomId"`
	} `json:"data"`
}

type RoomMessage struct {
	Type string `json:"type"`
	Data struct {
		Message string `json:"message"`
		RoomID  string `json:"roomId"`
		Host    string `json:"host"`
	} `json:"data"`
}

type RoomJoinedEvent struct {
	Type string `json:"type"`
	Data struct {
		Host   string `json:"host"`
		RoomID string `json:"roomId"`
	} `json:"data"`
}

type WelcomeMessage struct {
	Type string `json:"type"`
	Data struct {
		Host   string `json:"host"`
		RoomID string `json:"roomId"`
	} `json:"data"`
}

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

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		message = utils.CheckTypeOfMessage(string(message), c.conn.RemoteAddr().String()).([]byte)
		c.hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			fmt.Println("message inside writePump: ", string(message))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	// Rooms to store clients
	rooms map[string][]*Client // key is the room id and value is the list of clients

	mu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string][]*Client),
	}
}

type Message interface {
	GetType() string
}

func (m CreatedRoomMessage) GetType() string {
	return m.Type
}

func (m RoomJoinedEvent) GetType() string {
	return m.Type
}

func (m RoomMessage) GetType() string {
	return m.Type
}

func (m WelcomeMessage) GetType() string {
	return m.Type
}

var messageTypes = map[string]func() Message{
	"room-created":    func() Message { return &CreatedRoomMessage{} },
	"room-joined":     func() Message { return &RoomJoinedEvent{} },
	"room-message":    func() Message { return &RoomMessage{} },
	"welcome-message": func() Message { return &WelcomeMessage{} },
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				delete(h.rooms, client.conn.RemoteAddr().String())
				close(client.send)
			}
		case message := <-h.broadcast:
			fmt.Println("message inside broadcast: ", string(message))

			var rawMsg map[string]interface{}
			if err := json.Unmarshal(message, &rawMsg); err != nil {
				fmt.Println("error trying to unmarshal rawMsg: ", err)
				return
			}

			msgType, ok := rawMsg["type"].(string)
			if !ok {
				fmt.Println("error trying to cast msgType: ", ok)
				return
			}

			msgFactory, ok := messageTypes[msgType]
			if !ok {
				fmt.Println("error trying to get msgFactory: ", ok)
				return
			}

			msg := msgFactory()
			if err := json.Unmarshal(message, msg); err != nil {
				fmt.Println("error trying to unmarshal msg: ", err)
				return
			}
			host := ""
			var foundClient *Client
			switch msg := msg.(type) {
			case *CreatedRoomMessage:
				fmt.Println("CreatedRoomMessage: ", msg)
				messageTypes := []string{"room-created", "room-joined", "room-message"}
				if utils.Contains(messageTypes, msg.Type) {
					host = msg.Data.Host
					for client := range h.clients {
						if client.conn.RemoteAddr().String() == host {
							foundClient = client
							break
						}
					}
				}
			case *RoomJoinedEvent:
				fmt.Println("RoomJoinedEvent: ", msg)
				messageTypes := []string{"room-created", "room-joined", "room-message"}
				if utils.Contains(messageTypes, msg.Type) {
					host = msg.Data.Host
					for client := range h.clients {
						if client.conn.RemoteAddr().String() == host && client.hub.rooms[msg.Data.RoomID] != nil
						&& client.hub.rooms[msg.Data.RoomID] {
							if client.conn.RemoteAddr().String() == msg.Data.Host {

							}
							foundClient = client
							break
						}
					}
				}
			case *RoomMessage:
				fmt.Println("RoomMessage: ", msg)
				messageTypes := []string{"room-created", "room-joined", "room-message"}
				if utils.Contains(messageTypes, msg.Type) {
					host = msg.Data.Host
					for client := range h.clients {
						if client.conn.RemoteAddr().String() == host {
							foundClient = client
							break
						}
					}
				}
			}

			for client := range h.clients {
				incomingMessage := ""
				if client == foundClient && msg.GetType() != "room-created" {
					fmt.Println("found client, skipping message: ", client)
					continue
				}
				if msg.GetType() == "room-joined" {
					fmt.Println("room-joined if: ", msg.(*RoomJoinedEvent))
					h.mu.RLock()
					_, ok := h.rooms[msg.(*RoomJoinedEvent).Data.RoomID]
					h.mu.RUnlock()
					if ok {
						fmt.Println("room-joined if -> ok: ", msg.(*RoomJoinedEvent))
						h.mu.RLock()
						fmt.Println("currentRoom: ", h.rooms[msg.(*RoomJoinedEvent).Data.RoomID])
						h.mu.RUnlock()
						joinMessageForRoom := map[string]interface{}{
							"type": "welcome-message",
							"data": map[string]interface{}{
								"host":   msg.(*RoomJoinedEvent).Data.Host,
								"roomId": msg.(*RoomJoinedEvent).Data.RoomID,
							},
						}
						incomingMessageBytes, err := json.Marshal(joinMessageForRoom)
						if err != nil {
							fmt.Println("error marshaling incoming message: ", err)
							return
						}
						if client.conn.RemoteAddr().String() == msg.(*RoomJoinedEvent).Data.Host {
							client.hub.rooms[msg.(*RoomJoinedEvent).Data.RoomID] = append(client.hub.rooms[msg.(*RoomJoinedEvent).Data.RoomID], client)
						}
						h.mu.RLock()
						fmt.Println("room after add: ", h.rooms[msg.(*RoomJoinedEvent).Data.RoomID])
						h.mu.RUnlock()

						h.mu.RLock()
						for _, c := range h.rooms[msg.(*RoomJoinedEvent).Data.RoomID] {
							fmt.Println("client loop: ", c.conn.RemoteAddr().String())
							if c.conn.RemoteAddr().String() == msg.(*RoomJoinedEvent).Data.Host {
								continue
							}
							incomingMessage = string(incomingMessageBytes)
						}
						h.mu.RUnlock()
					}
				}

				if msg.GetType() == "room-created" {
					if client.conn.RemoteAddr().String() != msg.(*CreatedRoomMessage).Data.Host {
						continue
					}
					client.hub.rooms[msg.(*CreatedRoomMessage).Data.RoomID] = append(client.hub.rooms[msg.(*CreatedRoomMessage).Data.RoomID], client)
					fmt.Println("room created and client joined: ", client.hub.rooms)
					fmt.Println("room-created:client: ", client.conn.RemoteAddr().String())
					incomingMessageMap := map[string]interface{}{
						"type": "room-created",
						"data": map[string]interface{}{
							"host":   msg.(*CreatedRoomMessage).Data.Host,
							"roomId": msg.(*CreatedRoomMessage).Data.RoomID,
						},
					}
					incomingMessageBytes, err := json.Marshal(incomingMessageMap)
					if err != nil {
						fmt.Println("error marshaling incoming message: ", err)
						return
					}
					incomingMessage = string(incomingMessageBytes)
				}
				fmt.Println("Message: ", incomingMessage)
				select {
				case client.send <- []byte(incomingMessage):
				default:
					close(client.send)
					delete(h.clients, client)
					delete(h.rooms, client.conn.RemoteAddr().String())
				}
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handler(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}
	client.hub.register <- client
	go client.writePump()
	go client.readPump()
}

func main() {
	hub := NewHub()
	go hub.Run()
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handler(hub, w, r)
	})
	fmt.Printf("Server started at http://localhost:8080\n")
	http.ListenAndServe(":8080", nil)
}
