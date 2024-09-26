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
		RoomId   string   `json:"roomId,omitempty"`
		Id       string   `json:"id,omitempty"`
		Message  string   `json:"message,omitempty"`
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
			"data": "pong",
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
				"roomId": roomID,
				"host":   client,
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
				"roomId": message.Data.Id,
				"host":   client,
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
