package message

import (
	"strings"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// Message is a (simpler) abstraction over a WhatsApp message event.
type Message struct {
	Text      string
	PushName  string
	Sender    types.JID
	Recipient types.JID
	MessageID string
	IsMedia   bool
}

// New creates a new Message from a whatsmeow event, returning nil for irrelevant messages.
func New(evt *events.Message) *Message {
	// ignore messages from groups or channels.
	if evt.Info.Chat.Server != "s.whatsapp.net" {
		return nil
	}

	msg := &Message{
		Sender:    evt.Info.Sender,
		Recipient: evt.Info.Sender,
		MessageID: evt.Info.ID,
		PushName:  evt.Info.PushName,
	}

	msg.Text, msg.IsMedia = extractContent(evt)

	return msg
}

// extractContent safely pulls text or caption from various message types.
func extractContent(evt *events.Message) (text string, isMedia bool) {
	msg := evt.Message
	switch {
	case msg.GetConversation() != "":
		return msg.GetConversation(), false
	case msg.GetExtendedTextMessage() != nil:
		return msg.GetExtendedTextMessage().GetText(), false
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage().GetCaption(), true
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage().GetCaption(), true
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage().GetCaption(), true
	default:
		// any other type is considered media if it's not text-based.
		isMedia := msg.GetStickerMessage() != nil || msg.GetAudioMessage() != nil

		return "", isMedia
	}
}

func (m *Message) GetSenderID() string {
	return m.Sender.ToNonAD().String()
}

func (m *Message) GetText() string {
	return strings.TrimSpace(m.Text)
}

func (m *Message) GetPushName() string {
	return m.PushName
}

func (m *Message) HasMedia() bool {
	return m.IsMedia
}
