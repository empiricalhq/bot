package message

import (
	"strings"

	"go.mau.fi/whatsmeow/types/events"
)

type Message struct {
	Text      string
	PushName  string
	SenderID  string
	HasMedia  bool
	MediaType string // "image", "video", "document", "audio", "sticker"
}

func FromEvent(evt *events.Message) *Message {
	// Rule 1: Only listen to direct 1-on-1 messages.
	if evt.Info.Chat.Server != "s.whatsapp.net" {
		return nil
	}

	msg := &Message{
		SenderID: evt.Info.Chat.ToNonAD().String(),
		PushName: evt.Info.PushName,
	}

	var content string

	content, msg.HasMedia, msg.MediaType = extractContent(evt)
	msg.Text = strings.TrimSpace(content)

	// Rule 2: Ignore any message that does not contain usable text or media.
	if msg.Text == "" && !msg.HasMedia {
		return nil
	}

	return msg
}

func extractContent(evt *events.Message) (caption string, hasMedia bool, mediaType string) {
	msg := evt.Message

	switch {
	case msg.GetConversation() != "":
		return msg.GetConversation(), false, ""
	case msg.GetExtendedTextMessage() != nil:
		return msg.GetExtendedTextMessage().GetText(), false, ""
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage().GetCaption(), true, "image"
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage().GetCaption(), true, "document"
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage().GetCaption(), true, "video"
	case msg.GetAudioMessage() != nil:
		return "", true, "audio"
	case msg.GetStickerMessage() != nil:
		return "", true, "sticker"
	default:
		return "", false, ""
	}
}
