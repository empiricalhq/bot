package message

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const sendTimeout = 15 * time.Second

var ErrCannotSendEmptyMessage = errors.New("cannot send empty message")

// Sender is responsible for sending messages via the WhatsApp client.
type Sender struct {
	client *whatsmeow.Client
}

// NewSender creates a new message sender.
func NewSender(client *whatsmeow.Client) *Sender {
	return &Sender{client: client}
}

// SendText sends a text message to a recipient.
func (s *Sender) SendText(ctx context.Context, recipient types.JID, text string) error {
	if text == "" {
		return ErrCannotSendEmptyMessage
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()

	_, err := s.client.SendMessage(sendCtx, recipient, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return fmt.Errorf("failed to send text message to %s: %w", recipient, err)
	}

	return nil
}
