package message

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

type Sender struct {
	client *whatsmeow.Client
}

func NewSender(client *whatsmeow.Client) *Sender {
	return &Sender{client: client}
}

func (s *Sender) SendText(ctx context.Context, recipient types.JID, text string) error {
	_, err := s.client.SendMessage(ctx, recipient, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return fmt.Errorf("failed to send text message to %s: %w", recipient.String(), err)
	}

	return nil
}

func (s *Sender) SendImage(ctx context.Context, recipient types.JID, imageData []byte, caption string) error {
	resp, err := s.client.Upload(ctx, imageData, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("failed to upload image for %s: %w", recipient.String(), err)
	}

	imageMsg := &waE2E.ImageMessage{
		Mimetype:      proto.String("image/jpeg"),
		URL:           &resp.URL,
		DirectPath:    &resp.DirectPath,
		MediaKey:      resp.MediaKey,
		FileEncSHA256: resp.FileEncSHA256,
		FileSHA256:    resp.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(imageData))),
	}
	if caption != "" {
		imageMsg.Caption = proto.String(caption)
	}

	_, err = s.client.SendMessage(ctx, recipient, &waE2E.Message{
		ImageMessage: imageMsg,
	})
	if err != nil {
		return fmt.Errorf("failed to send image message to %s: %w", recipient.String(), err)
	}

	return nil
}

func (s *Sender) SendDocument(ctx context.Context, recipient types.JID, documentData []byte, filename, mimetype string) error {
	resp, err := s.client.Upload(ctx, documentData, whatsmeow.MediaDocument)
	if err != nil {
		return fmt.Errorf("failed to upload document for %s: %w", recipient.String(), err)
	}

	documentMsg := &waE2E.DocumentMessage{
		Mimetype:      proto.String(mimetype),
		URL:           &resp.URL,
		DirectPath:    &resp.DirectPath,
		MediaKey:      resp.MediaKey,
		FileEncSHA256: resp.FileEncSHA256,
		FileSHA256:    resp.FileSHA256,
		FileName:      proto.String(filename),
		FileLength:    proto.Uint64(uint64(len(documentData))),
	}

	_, err = s.client.SendMessage(ctx, recipient, &waE2E.Message{
		DocumentMessage: documentMsg,
	})
	if err != nil {
		return fmt.Errorf("failed to send document message to %s: %w", recipient.String(), err)
	}

	return nil
}

func (s *Sender) SendReaction(ctx context.Context, recipient types.JID, messageID, emoji string) error {
	participantJID := recipient
	/*
		if recipient.Server == types.GroupServer {
			// TODO: handle group reactions properly if needed
		}
	*/

	_, err := s.client.SendMessage(ctx, recipient, &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waCommon.MessageKey{
				RemoteJID:   proto.String(recipient.String()),
				FromMe:      proto.Bool(true),
				ID:          proto.String(messageID),
				Participant: proto.String(participantJID.String()),
			},
			Text: proto.String(emoji),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send reaction message to %s for ID %s: %w", recipient.String(), messageID, err)
	}

	return nil
}
