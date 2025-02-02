package utils

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/google/uuid"
)

type RoomData interface {
	GetHost() string
	GetRoomID() string
	GetTopic() string
	GetTotalRounds() int
	GetPlayersInfo() []PlayerInfo
}

type PlayerInfo struct {
	Nickname string `json:"nickname"`
	Client   string `json:"client"`
}

type RoomDataImpl struct {
	Host        string       `json:"host"`
	RoomID      string       `json:"roomId"`
	Topic       string       `json:"topic"`
	TotalRounds int          `json:"totalRounds"`
	PlayersInfo []PlayerInfo `json:"playersInfo"`
}

func (r RoomDataImpl) GetHost() string {
	return r.Host
}

func (r RoomDataImpl) GetRoomID() string {
	return r.RoomID
}

func (r RoomDataImpl) GetTopic() string {
	return r.Topic
}

func (r RoomDataImpl) GetTotalRounds() int {
	return r.TotalRounds
}

func (r RoomDataImpl) GetPlayersInfo() []PlayerInfo {
	return r.PlayersInfo
}

type BoolString bool

func (b *BoolString) UnmarshalJSON(data []byte) error {
	str := string(data)
	fmt.Println("str: ", str)
	switch str {
	case `"true"`:
		*b = true
	case `"false"`:
		*b = false
	default:
		*b = false
	}
	return nil
}

type Answer struct {
	AnswerA string `json:"answer_a,omitempty"`
	AnswerB string `json:"answer_b,omitempty"`
	AnswerC string `json:"answer_c,omitempty"`
	AnswerD string `json:"answer_d,omitempty"`
	AnswerE string `json:"answer_e,omitempty"`
	AnswerF string `json:"answer_f,omitempty"`
}

type CorrectAnswers struct {
	AnswerACorrect bool `json:"answer_a_correct"`
	AnswerBCorrect bool `json:"answer_b_correct"`
	AnswerCCorrect bool `json:"answer_c_correct"`
	AnswerDCorrect bool `json:"answer_d_correct"`
	AnswerECorrect bool `json:"answer_e_correct"`
	AnswerFCorrect bool `json:"answer_f_correct"`
}

type QuizzItem struct {
	Id                     *string        `json:"id,omitempty"`
	Question               *string        `json:"question,omitempty"`
	Description            *string        `json:"description,omitempty"`
	Answers                Answer         `json:"answers,omitempty"`
	MultipleCorrectAnswers *bool          `json:"multiple_correct_answers,omitempty"`
	CorrectAnswers         CorrectAnswers `json:"correct_answers,omitempty"`
	CorrectAnswer          *string        `json:"correct_answer,omitempty"`
	Explanation            *string        `json:"explanation,omitempty"`
	Tip                    *string        `json:"tip,omitempty"`
	Tags                   *[]Tag         `json:"tags,omitempty"`
	Category               *string        `json:"category,omitempty"`
	Difficulty             *string        `json:"difficulty,omitempty"`
}

type Tag struct {
	Name string `json:"name,omitempty"`
}

type Message struct {
	Type string `json:"type"`
	Data struct {
		RoomId  string `json:"roomId,omitempty"`
		Id      string `json:"id,omitempty"`
		Message struct {
			Question string      `json:"question,omitempty"`
			Answer   string      `json:"answer,omitempty"`
			Round    int         `json:"round,omitempty"`
			Topic    string      `json:"topic,omitempty"`
			RoundQtt int         `json:"roundQtt,omitempty"`
			Quizz    []QuizzItem `json:"quizz,omitempty"`
		} `json:"message,omitempty"`
		Host     string   `json:"host,omitempty"`
		Receiver string   `json:"receiver,omitempty"`
		Users    []string `json:"users,omitempty"`
		Username string   `json:"username,omitempty"`
	} `json:"data,omitempty"`
}

type RoomMessage struct {
	Type string `json:"type"`
	Data struct {
		Message string `json:"message"`
		RoomID  string `json:"roomId"`
		Sender  string `json:"sender"`
	} `json:"data"`
}

type JoinRoom struct {
	Type string `json:"type"`
	Data struct {
		RoomId   string `json:"roomId"`
		Nickname string `json:"nickname"`
	} `json:"data"`
}

type CreateRoom struct {
	Type string `json:"type"`
	Data struct {
		Name     string `json:"name"`
		Topic    string `json:"topic"`
		RoundQtt int    `json:"roundQtt"`
		Nickname string `json:"nickname"`
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

func CheckTypeOfMessage(incomingMessage []byte, client string) ([]byte, bool) {
	fmt.Println("incoming message:CheckTypeOfMessage: ", incomingMessage)
	var messageType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(incomingMessage, &messageType); err != nil {
		fmt.Println("error trying to unmarshal messageType on CheckTypeOfMessage: ", err)
		return nil, false
	}
	fmt.Println("messageType:CheckTypeOfMessage: ", messageType)
	fmt.Println("messageType:CONTENT: ", string(incomingMessage))

	switch messageType.Type {
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
			return nil, false
		}
		return encodedMessage, true
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
			return nil, false
		}
		return encodedMessage, true
	case "create-room":
		var createRoomMsg CreateRoom
		if err := json.Unmarshal(incomingMessage, &createRoomMsg); err != nil {
			log.Default().Println("error Unmarshal create-room: ", err)
			return nil, false
		}
		roomID := uuid.New().String()
		event := map[string]interface{}{
			"type": "room-created",
			"data": map[string]interface{}{
				"roomId":      roomID,
				"host":        client,
				"name":        createRoomMsg.Data.Name,
				"topic":       createRoomMsg.Data.Topic,
				"totalRounds": createRoomMsg.Data.RoundQtt,
				"nickname":    createRoomMsg.Data.Nickname,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal create-room: ", err)
			return nil, false
		}
		return encodedMessage, true

	case "join-room":
		var joinRoomMsg JoinRoom
		if err := json.Unmarshal(incomingMessage, &joinRoomMsg); err != nil {
			log.Default().Println("error Unmarshal join-room: ", err)
			return nil, false
		}
		event := map[string]interface{}{
			"type": "room-joined",
			"data": map[string]interface{}{
				"roomId":   joinRoomMsg.Data.RoomId,
				"client":   client,
				"nickname": joinRoomMsg.Data.Nickname,
			},
		}
		encodedMessage, err := json.Marshal(event)
		if err != nil {
			log.Default().Println("error Marshal join-room: ", err)
			return nil, false
		}
		return encodedMessage, true
	case "room-message":
		var roomMessage RoomMessage
		if err := json.Unmarshal([]byte(incomingMessage), &roomMessage); err != nil {
			log.Default().Println("error Unmarshal room-message: ", err)
			return nil, false
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
			return nil, false
		}
		return encodedMessage, true
	case "start-game":
		event := map[string]interface{}{
			"type": "game-started",
			"data": map[string]interface{}{},
		}
		encodedMessage := EncodeMessage(event)
		if encodedMessage == nil {
			log.Default().Println("error Marshal start-game")
			return nil, false
		}
		return encodedMessage, true
	case "save-topic":
		var saveTopicMsg SaveTopic
		if err := json.Unmarshal(incomingMessage, &saveTopicMsg); err != nil {
			log.Default().Println("error Unmarshal save-topic: ", err)
			return nil, false
		}
		log.Println("saveTopicMsg:switch-case: ", saveTopicMsg)
		event := map[string]interface{}{
			"type": "topic-saved",
			"data": map[string]interface{}{
				"topic":     saveTopicMsg.Data.Topic,
				"roomId":    saveTopicMsg.Data.RoomID,
				"topicData": saveTopicMsg.Data.TopicData,
			},
		}
		encodedMessage := EncodeMessage(event)
		if encodedMessage == nil {
			log.Default().Println("error Marshal save-topic")
			return nil, false
		}
		fmt.Println("encodedMessage: ", encodedMessage)
		return encodedMessage, true
	}
	return nil, false
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

func (m SaveTopic) GetType() string {
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
		Nickname    string               `json:"nickname,omitempty"`
	} `json:"data"`
}

type RoomJoinedEvent struct {
	Type string `json:"type"`
	Data struct {
		Client   string `json:"client"`
		RoomID   string `json:"roomId"`
		Nickname string `json:"nickname"`
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

type SaveTopic struct {
	Type string `json:"type"`
	Data struct {
		Topic     string      `json:"topic"`
		RoomID    string      `json:"roomId"`
		TopicData []QuizzItem `json:"topicData"`
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
	"topic-saved":     func() MessageType { return &SaveTopic{} },
	"pong":            func() MessageType { return &Pong{} },
}

func ProcessMessage(message []byte) (MessageType, error) {
	var raw_msg map[string]interface{}
	if err := json.Unmarshal(message, &raw_msg); err != nil {
		return nil, fmt.Errorf("error trying to unmarshal raw_msg: %v", err)
	}

	msg_type, ok := raw_msg["type"].(string)
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
