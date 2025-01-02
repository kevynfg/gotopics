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

const (
	// Time allowed to write a message to the peer.
	writeWait = 20 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 2048
)

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

	// Room data to store room information
	roomData map[string]interface{}

	// Mutex to protect the rooms map
	mu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string][]*Client),
		roomData:   make(map[string]interface{}),
	}
}

func (h *Hub) Run() {
	var incoming_message string
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			client_info := map[string]interface{}{
				"type":   "client-login",
				"client": client.conn.RemoteAddr().String(),
			}
			incoming_message_bytes, err := json.Marshal(client_info)
			if err != nil {
				fmt.Println("error marshaling incoming message for user info: ", err)
				return
			}
			incoming_message = string(incoming_message_bytes)
			fmt.Println("incoming_message:Run: ", incoming_message)
			client.send <- []byte(incoming_message)
		case client := <-h.unregister:
			fmt.Println("client inside unregister: ", client)
			if _, ok := h.clients[client]; ok {
				fmt.Println("unregistering: ", client)
				delete(h.clients, client)
				delete(h.rooms, client.conn.RemoteAddr().String())
				close(client.send)
			}
		case message := <-h.broadcast:
			fmt.Println("message inside broadcast: ", string(message))

			msg_type, err := utils.ProcessMessage(message)
			if err != nil {
				fmt.Println("error processing message: ", err)
				return
			}

			switch msg := msg_type.(type) {
			case *utils.CreatedRoomMessage:
				fmt.Println("CreatedRoomMessage: ", msg)
				h.handleCreatedRoomMessage(msg)
			case *utils.RoomJoinedEvent:
				fmt.Println("RoomJoinedEvent: ", msg)
				h.handleRoomJoinedEvent(msg)
			case *utils.RoomMessage:
				fmt.Println("RoomMessage: ", msg)
				h.handleRoomMessage(msg, message)
			case *utils.SaveTopic:
				fmt.Println("SaveTopic: ", msg)
				h.handleSaveTopic(msg, message)
			}
		}
	}
}

func (h *Hub) handleCreatedRoomMessage(msg *utils.CreatedRoomMessage) {
	host := msg.Data.Host
	var found_client *Client
	for client := range h.clients {
		if client.conn.RemoteAddr().String() == host {
			found_client = client
			break
		}
	}

	if found_client != nil {
		found_client.hub.rooms[msg.Data.RoomID] = append(found_client.hub.rooms[msg.Data.RoomID], found_client)
		fmt.Println("room created and client joined: ", found_client.hub.rooms)
		fmt.Println("room-created:client: ", found_client.conn.RemoteAddr().String())
		fmt.Println("room-created:data: ", msg.Data)
		incoming_message_map := map[string]interface{}{
			"type":    "room-created",
			"host":    msg.Data.Host,
			"roomId":  msg.Data.RoomID,
			"topic":   msg.Data.Topic,
			"rounds":  msg.Data.TotalRounds,
			"clients": getClientAddresses(h.rooms[msg.Data.RoomID]),
		}
		if _, ok := h.roomData[msg.Data.RoomID]; !ok {
			h.roomData[msg.Data.RoomID] = []interface{}{}
		}
		h.roomData[msg.Data.RoomID] = append(h.roomData[msg.Data.RoomID].([]interface{}),
			map[string]interface{}{
				"host":   msg.Data.Host,
				"roomId": msg.Data.RoomID,
				"topic":  msg.Data.Topic,
				"rounds": msg.Data.TotalRounds,
			})
		incoming_message_bytes, err := json.Marshal(incoming_message_map)
		if err != nil {
			fmt.Println("error marshaling incoming message handleCreatedRoomMessage: ", err)
			return
		}
		incoming_message := string(incoming_message_bytes)
		h.sendMessageToRoom(msg.Data.RoomID, incoming_message)
	}
}

func (h *Hub) handleRoomJoinedEvent(msg *utils.RoomJoinedEvent) {
	h.mu.RLock()
	_, ok := h.rooms[msg.Data.RoomID]
	h.mu.RUnlock()
	if ok {
		for client := range h.clients {
			if client.conn.RemoteAddr().String() == msg.Data.Client {
				client.hub.rooms[msg.Data.RoomID] = append(client.hub.rooms[msg.Data.RoomID], client)
				break
			}
		}
		join_message_for_room := map[string]interface{}{
			"type": "welcome-message",
			"data": map[string]interface{}{
				"client":      msg.Data.Client,
				"roomId":      msg.Data.RoomID,
				"message":     "Welcome to the room",
				"clients":     getClientAddresses(h.rooms[msg.Data.RoomID]),
				"isNewMember": true,
				"roomData":    h.roomData[msg.Data.RoomID],
			},
		}
		incoming_message_bytes, err := json.Marshal(join_message_for_room)
		if err != nil {
			fmt.Println("error marshaling incoming message handleRoomJoinedEvent: ", err)
			return
		}
		incoming_message := string(incoming_message_bytes)
		h.sendMessageToRoom(msg.Data.RoomID, incoming_message)
	}
}

func (h *Hub) handleRoomMessage(msg *utils.RoomMessage, original_message []byte) {
	h.mu.RLock()
	_, ok := h.rooms[msg.Data.RoomID]
	h.mu.RUnlock()
	if ok {
		incoming_message := string(original_message)
		h.sendMessageToRoom(msg.Data.RoomID, incoming_message)
	}
}

func (h *Hub) handleSaveTopic(msg *utils.SaveTopic, original_message []byte) {
	h.mu.RLock()
	_, ok := h.rooms[msg.Data.RoomID]
	h.mu.RUnlock()
	if ok {
		incoming_message := string(original_message)
		h.sendMessageToRoom(msg.Data.RoomID, incoming_message)
	}
}

func (h *Hub) sendMessageToRoom(roomId string, message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.rooms[roomId] {
		select {
		case client.send <- []byte(message):
		default:
			close(client.send)
			delete(h.clients, client)
			delete(h.rooms, client.conn.RemoteAddr().String())
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
	go client.readPump()
	go client.writePump()
}

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
		var valid bool
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error:readPump: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		message, valid = utils.CheckTypeOfMessage(message, c.conn.RemoteAddr().String())
		if !valid {
			log.Println("invalid message:readPump: ", string(message))
			return
		}
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
			log.Println("message:writePump: ", string(message))
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				err := c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				// log.Fatalf("error writing close message: %v", err)
				log.Println("error writing message", message, err)
				c.conn.Close()
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				fmt.Println("error getting writer: ", err)
				return
			}
			w.Write(message)
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				log.Println("error closing writer: ", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("error writing ping message: ", err)
				return
			}
		}
	}
}

func getClientAddresses(clients []*Client) []string {
	addresses := make([]string, len(clients))
	for i, client := range clients {
		addresses[i] = client.conn.RemoteAddr().String()
	}
	return addresses
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
