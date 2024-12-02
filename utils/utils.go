package utils

//import from main package
import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
)

type Message struct {
	Type string `json:"type"`
	Data struct {
		RoomId  string `json:"roomId,omitempty"`
		Id      string `json:"id,omitempty"`
		Message struct {
			Question string `json:"question,omitempty"`
			Answer   string `json:"answer,omitempty"`
			Round    int    `json:"round,omitempty"`
			Topic    string `json:"topic,omitempty"`
			RoundQtt int    `json:"roundQtt,omitempty"`
		} `json:"message,omitempty"`
		Host     string   `json:"host,omitempty"`
		Receiver string   `json:"receiver,omitempty"`
		Users    []string `json:"users,omitempty"`
		Username string   `json:"username,omitempty"`
	} `json:"data"`
}

type RoomMessage struct {
	Type string `json:"type"`
	Data struct {
		Message string `json:"message"`
		RoomID  string `json:"roomId"`
		Sender  string `json:"sender"`
	} `json:"data"`
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func CheckTypeOfMessage(incomingMessage string, client string) interface{} {
	fmt.Println("incoming message: ", incomingMessage)
	var message Message
	if err := json.Unmarshal([]byte(incomingMessage), &message); err != nil {
		fmt.Println("error trying to unmarshal message on CheckTypeOfMessage: ", err)
		return false
	}
	switch message.Type {
	case "ping":
		event := map[string]interface{}{
			"type": "pong",
			"data": map[string]interface{}{
				"message": "pong",
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal ping: ", err)
			return false
		}
		return encodedMessage
	case "join-app":
		event := map[string]interface{}{
			"type": "joined",
			"data": map[string]interface{}{
				"message":    "Event to join app and return user connection",
				"connection": client,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			return false
		}
		return encodedMessage
	case "create-room":
		roomID := uuid.New().String()
		event := map[string]interface{}{
			"type": "room-created",
			"data": map[string]interface{}{
				"roomId":      roomID,
				"host":        client,
				"topic":       message.Data.Message.Topic,
				"totalRounds": message.Data.Message.RoundQtt,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal create-room: ", err)
			return false
		}
		return encodedMessage

	case "join-room":
		event := map[string]interface{}{
			"type": "room-joined",
			"data": map[string]interface{}{
				"roomId": message.Data.RoomId,
				"client": client,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal join-room: ", err)
			return false
		}
		return encodedMessage
	case "room-message":
		var roomMessage RoomMessage
		if err := json.Unmarshal([]byte(incomingMessage), &roomMessage); err != nil {
			log.Default().Println("error Unmarshal room-message: ", err)
			return false
		}
		event := map[string]interface{}{
			"type": "room-message",
			"data": map[string]interface{}{
				"message": roomMessage.Data.Message,
				"sender":  client,
				"roomId":  roomMessage.Data.RoomID,
				"host":    client,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal room-message: ", err)
			return false
		}
		return encodedMessage
	case "start-game":
		event := map[string]interface{}{
			"type": "game-started",
			"data": map[string]interface{}{
				"roomId": message.Data.RoomId,
			},
		}
		encodedMessage := EncodeMessage(event)
		if encodedMessage == nil {
			log.Default().Println("error Marshal start-game")
			return false
		}
		return encodedMessage
	}
	return interface{}(nil)
}

func EncodeMessage(message map[string]interface{}) []byte {
	encodedMessage, err := json.Marshal(message)
	if err != nil {
		log.Default().Println("error Marshal: ", err)
		return nil
	}
	return encodedMessage
}

func EncodeMessageToBytes(message string) []byte {
	encodedMessage, err := json.Marshal(message)
	if err != nil {
		log.Default().Println("error Marshal: ", err)
		return nil
	}
	return encodedMessage
}

func ContainsClient(clients []string, client string) bool {
	for _, c := range clients {
		if c == client {
			return true
		}
	}
	return false
}

type MessageType interface {
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

func (m JoinApp) GetType() string {
	return m.Type
}

func (m Pong) GetType() string {
	return m.Type
}

type Question struct {
	Answer string `json:"answer,omitempty"`
	Player string `json:"player,omitempty"`
}

type CreatedRoomMessage struct {
	Type string `json:"type"`
	Data struct {
		Host        string               `json:"host"`
		RoomID      string               `json:"roomId"`
		Topic       string               `json:"topic,omitempty"`
		Round       int                  `json:"round,omitempty"`
		Question    map[string]*Question `json:"question,omitempty"`
		TotalRounds int                  `json:"totalRounds,omitempty"`
	} `json:"data"`
}

type RoomJoinedEvent struct {
	Type string `json:"type"`
	Data struct {
		Client string `json:"client"`
		RoomID string `json:"roomId"`
	} `json:"data"`
}

type WelcomeMessage struct {
	Type string `json:"type"`
	Data struct {
		Host   string `json:"host,omitempty"`
		RoomID string `json:"roomId,omitempty"`
	} `json:"data"`
}

type JoinApp struct {
	Type string `json:"type"`
	Data struct {
		Message    string `json:"message"`
		Connection string `json:"connection"`
	} `json:"data"`
}

type Pong struct {
	Type string `json:"type"`
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

var messageTypes = map[string]func() MessageType{
	"room-created":    func() MessageType { return &CreatedRoomMessage{} },
	"room-joined":     func() MessageType { return &RoomJoinedEvent{} },
	"room-message":    func() MessageType { return &RoomMessage{} },
	"welcome-message": func() MessageType { return &WelcomeMessage{} },
	"join-app":        func() MessageType { return &JoinApp{} },
	"pong":            func() MessageType { return &Pong{} },
}

func ProcessMessage(message []byte) (MessageType, error) {
	var raw_msg map[string]interface{}
	if err := json.Unmarshal(message, &raw_msg); err != nil {
		return nil, fmt.Errorf("error trying to unmarshal raw_msg: %v", err)
	}

	msg_type, ok := raw_msg["type"].(string)
	fmt.Println("ProcessMessage: ", msg_type)
	if !ok {
		return nil, fmt.Errorf("error trying to cast msg_type: %v", ok)
	}

	msg_factory, ok := messageTypes[msg_type]
	if !ok {
		return nil, fmt.Errorf("error trying to get msg_factory: %v", ok)
	}

	msg := msg_factory()
	if err := json.Unmarshal(message, msg); err != nil {
		return nil, fmt.Errorf("error trying to unmarshal msg: %v", err)
	}

	return msg, nil
}
