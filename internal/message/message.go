package message

import (
	"fmt"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	Text      string
	Sender    types.JID
	Recipient types.JID
	MessageID string
}

func New(evt *events.Message) *Message {
	if evt.Info.IsGroup {
		return nil
	}

	msg := &Message{
		Sender:    evt.Info.Sender,
		Recipient: evt.Info.Sender,
		MessageID: evt.Info.ID,
	}

	// extract text content from various message types
	if evt.Message.GetConversation() != "" {
		msg.Text = evt.Message.GetConversation()
	} else if extText := evt.Message.GetExtendedTextMessage(); extText != nil {
		msg.Text = extText.GetText()
	} else if img := evt.Message.GetImageMessage(); img != nil {
		msg.Text = img.GetCaption()
	}

	return msg
}

func (m *Message) GetSenderID() string {
	return m.Sender.ToNonAD().String()
}

func (m *Message) GetText() string {
	return m.Text
}

func (m *Message) String() string {
	return fmt.Sprintf("ID: %s, From: %s, Text: %q", m.MessageID, m.GetSenderID(), m.Text)
}
