package message

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	Text      string
	PushName  string
	Sender    types.JID
	Recipient types.JID
	MessageID string
}

func New(evt *events.Message) *Message {
	// early return for non-direct messages
	if evt.Info.Chat.Server != "s.whatsapp.net" {
		return nil
	}

	msg := &Message{
		Sender:    evt.Info.Sender,
		Recipient: evt.Info.Sender,
		MessageID: evt.Info.ID,
		PushName:  evt.Info.PushName,
	}

	msg.Text = extractTextContent(evt)
	msg.Text = strings.TrimSpace(msg.Text)

	return msg
}

func extractTextContent(evt *events.Message) string {
	if text := evt.Message.GetConversation(); text != "" {
		return text
	}

	if extText := evt.Message.GetExtendedTextMessage(); extText != nil {
		return extText.GetText()
	}

	if img := evt.Message.GetImageMessage(); img != nil {
		return img.GetCaption()
	}

	if doc := evt.Message.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}

	if video := evt.Message.GetVideoMessage(); video != nil {
		return video.GetCaption()
	}

	return ""
}

func (m *Message) GetSenderID() string {
	return m.Sender.ToNonAD().String()
}

func (m *Message) GetText() string {
	return m.Text
}

func (m *Message) GetPushName() string {
	return m.PushName
}

func (m *Message) IsEmpty() bool {
	return strings.TrimSpace(m.Text) == ""
}

func (m *Message) String() string {
	return fmt.Sprintf("ID: %s, From: %s, Name: %q, Text: %q", m.MessageID, m.GetSenderID(), m.PushName, m.Text)
}
