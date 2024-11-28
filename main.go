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
	maxMessageSize = 512
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

func (h *Hub) Run() {
	var incomingMessage string
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			clientInfo := map[string]interface{}{
				"type": "client-login",
				"data": map[string]interface{}{
					"client": client.conn.RemoteAddr().String(),
				},
			}
			incomingMessageBytes, err := json.Marshal(clientInfo)
			if err != nil {
				fmt.Println("error marshaling incoming message for user info: ", err)
				return
			}
			incomingMessage = string(incomingMessageBytes)
			client.send <- []byte(incomingMessage)
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

			msgType, err := utils.ProcessMessage(message)
			if err != nil {
				fmt.Println("error processing message: ", err)
				return
			}

			host := ""
			var foundClient *Client
			switch msg := msgType.(type) {
			case *utils.CreatedRoomMessage:
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
			case *utils.RoomJoinedEvent:
				fmt.Println("RoomJoinedEvent: ", msg)
				messageTypes := []string{"room-created", "room-joined", "room-message"}
				if utils.Contains(messageTypes, msg.Type) {
					host = msg.Data.Host
					for client := range h.clients {
						if client.conn.RemoteAddr().String() == host && client.hub.rooms[msg.Data.RoomID] != nil &&
							utils.ContainsClient(getClientAddresses(client.hub.rooms[msg.Data.RoomID]), client.conn.RemoteAddr().String()) {
							if client.conn.RemoteAddr().String() == msg.Data.Host {
								continue
							}
							foundClient = client
							break
						}
					}
				}
				// case *utils.RoomMessage:
				// 	fmt.Println("RoomMessage: ", msg)
				// 	messageTypes := []string{"room-created", "room-joined", "room-message"}
				// 	if utils.Contains(messageTypes, msg.Type) {
				// 		host = msg.Data.Sender
				// 		for client := range h.clients {
				// 			if client.conn.RemoteAddr().String() == host {
				// 				foundClient = client
				// 				break
				// 			}
				// 		}
				// 	}
			}

			for client := range h.clients {
				if client == foundClient && msgType.GetType() != "room-created" {
					fmt.Println("found client, skipping message: ", client)
					continue
				}
				if msgType.GetType() == "room-joined" {
					fmt.Println("room-joined if: ", msgType.(*utils.RoomJoinedEvent).Data.RoomID)
					h.mu.RLock()
					_, ok := h.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID]
					fmt.Println("room-joined if -> res: ", ok)
					h.mu.RUnlock()
					if ok {
						if client.conn.RemoteAddr().String() == msgType.(*utils.RoomJoinedEvent).Data.Host {
							client.hub.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID] = append(client.hub.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID], client)
						}
						joinMessageForRoom := map[string]interface{}{
							"type": "welcome-message",
							"data": map[string]interface{}{
								"host":        msgType.(*utils.RoomJoinedEvent).Data.Host,
								"roomId":      msgType.(*utils.RoomJoinedEvent).Data.RoomID,
								"message":     "Welcome to the room",
								"clients":     getClientAddresses(h.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID]),
								"isNewMember": true,
							},
						}
						incomingMessageBytes, err := json.Marshal(joinMessageForRoom)
						if err != nil {
							fmt.Println("error marshaling incoming message: ", err)
							return
						}
						h.mu.RLock()
						fmt.Println("room after add: ", h.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID])
						h.mu.RUnlock()

						h.mu.RLock()
						for _, c := range h.rooms[msgType.(*utils.RoomJoinedEvent).Data.RoomID] {
							fmt.Println("client loop: ", c.conn.RemoteAddr().String())
							if c.conn.RemoteAddr().String() == msgType.(*utils.RoomJoinedEvent).Data.Host {
								continue
							}
							incomingMessage = string(incomingMessageBytes)
						}
						h.mu.RUnlock()
					}
				}

				if msgType.GetType() == "room-created" {
					if client.conn.RemoteAddr().String() != msgType.(*utils.CreatedRoomMessage).Data.Host {
						continue
					}
					client.hub.rooms[msgType.(*utils.CreatedRoomMessage).Data.RoomID] = append(client.hub.rooms[msgType.(*utils.CreatedRoomMessage).Data.RoomID], client)
					fmt.Println("room created and client joined: ", client.hub.rooms)
					fmt.Println("room-created:client: ", client.conn.RemoteAddr().String())
					incomingMessageMap := map[string]interface{}{
						"type": "room-created",
						"data": map[string]interface{}{
							"host":   msgType.(*utils.CreatedRoomMessage).Data.Host,
							"roomId": msgType.(*utils.CreatedRoomMessage).Data.RoomID,
						},
					}
					incomingMessageBytes, err := json.Marshal(incomingMessageMap)
					if err != nil {
						fmt.Println("error marshaling incoming message: ", err)
						return
					}
					incomingMessage = string(incomingMessageBytes)
				}

				if msgType.GetType() == "room-message" {
					fmt.Println("room-message: ", msgType.(*utils.RoomMessage).Data.RoomID)
					h.mu.RLock()
					_, ok := h.rooms[msgType.(*utils.RoomMessage).Data.RoomID]
					fmt.Println("room-message -> res: ", ok)
					h.mu.RUnlock()
					if ok {
						h.mu.RLock()
						for _, c := range h.rooms[msgType.(*utils.RoomMessage).Data.RoomID] {
							fmt.Println("client loop: ", c.conn.RemoteAddr().String())
							// if c.conn.RemoteAddr().String() == msgType.(*utils.RoomMessage).Data.Host {
							// 	continue
							// }
							if !utils.Contains(getClientAddresses(h.rooms[msgType.(*utils.RoomMessage).Data.RoomID]), client.conn.RemoteAddr().String()) {
								continue
							}
							incomingMessage = string(message)
						}
						h.mu.RUnlock()
					}
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
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
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
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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
