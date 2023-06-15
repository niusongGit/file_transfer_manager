package connTcpMessage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

type Message struct {
	MID       uint64
	Sender    string
	Recipient string
	Data      []byte
	ErrNote   string
}

type MessageHandler func(ctx context.Context, message *Message) ([]byte, error)

func NewMessage(mid uint64, sender string, recipient string, d []byte, e string) *Message {
	return &Message{
		MID:       mid,
		Sender:    sender,
		Recipient: recipient,
		Data:      d,
		ErrNote:   e,
	}
}

func (m *Message) ParseMessage(d []byte) (*Message, error) {
	decoder := json.NewDecoder(bytes.NewBuffer(d))
	decoder.UseNumber()
	err := decoder.Decode(m)
	if err != nil {
		fmt.Println(err)
	}
	return m, err
}

func (m *Message) SerializationMessage() ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		fmt.Println(err)
	}
	return b, err
}

// 回复消息统一Handers
func Recv_rev(ctx context.Context, message *Message) ([]byte, error) {
	if len(message.ErrNote) > 0 {
		return message.Data, errors.New(message.ErrNote)
	}
	return message.Data, nil
}
