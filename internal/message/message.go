package message

import (
	"strings"

	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	Text     string
	PushName string
	SenderID string
	HasMedia bool
}

func FromEvent(evt *events.Message) *Message {
	// Rule 1: Only listen to direct 1-on-1 messages.
	if evt.Info.Chat.Server != "s.whatsapp.net" {
		return nil
	}

	msg := &Message{
		SenderID: evt.Info.Sender.ToNonAD().String(),
		PushName: evt.Info.PushName,
	}

	msg.Text, msg.HasMedia = extractContent(evt)
	msg.Text = strings.TrimSpace(msg.Text)

	// Rule 2: Ignore any message that does not contain usable text.
	// This includes stickers, audio messages, and media sent without a caption.
	if msg.Text == "" {
		return nil
	}

	return msg
}

func extractContent(evt *events.Message) (string, bool) {
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
		// Other message types are considered media
		isMedia := msg.GetStickerMessage() != nil || msg.GetAudioMessage() != nil

		return "", isMedia
	}
}
