package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	"modernc.org/sqlite"

	"whatsbot/internal/logger"
)

type Client struct {
	*whatsmeow.Client
	logger logger.Logger
}

func NewClient(ctx context.Context, db *sql.DB, logger logger.Logger) (*Client, error) {
	container := sqlstore.NewWithDB(db, "sqlite3", newWhatsmeowLogger(logger))
	err := container.Upgrade(ctx)
	if err != nil {
		return nil, err
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		// Expected on first run
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code() != 1 {
			logger.Warn("Could not get first device", "error", err)
		}
	}

	client := whatsmeow.NewClient(device, newWhatsmeowLogger(logger))

	return &Client{
		Client: client,
		logger: logger,
	}, nil
}

func (c *Client) SendText(ctx context.Context, to, text string) error {
	if text == "" {
		return errors.New("cannot send empty message")
	}

	jid, err := types.ParseJID(to)
	if err != nil {
		return err
	}

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err = c.Client.SendMessage(sendCtx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})

	return err
}

func LoginWithQR(client *whatsmeow.Client, logger logger.Logger) error {
	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		return err
	}

	err = client.Connect()
	if err != nil {
		return err
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "success":
			logger.Info("QR login successful")
			return nil
		case "timeout":
			return errors.New("QR login timed out")
		}
	}

	return errors.New("QR channel closed unexpectedly")
}

// whatsmeowLogger adapts our logger to whatsmeow's interface
type whatsmeowLogger struct {
	logger logger.Logger
}

func newWhatsmeowLogger(logger logger.Logger) *whatsmeowLogger {
	return &whatsmeowLogger{logger: logger}
}

func (w *whatsmeowLogger) Errorf(msg string, args ...interface{}) {
	w.logger.Error(msg, args...)
}

func (w *whatsmeowLogger) Warnf(msg string, args ...interface{}) {
	w.logger.Warn(msg, args...)
}

func (w *whatsmeowLogger) Infof(msg string, args ...interface{}) {
	w.logger.Info(msg, args...)
}

func (w *whatsmeowLogger) Debugf(msg string, args ...interface{}) {
	w.logger.Debug(msg, args...)
}

func (w *whatsmeowLogger) Sub(module string) waLog.Logger {
	return &whatsmeowLogger{
		logger: w.logger,
	}
}
