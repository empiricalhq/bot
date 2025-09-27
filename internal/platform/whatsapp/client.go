package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
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
)

const sendMessageTimeout = 15 * time.Second

type Client struct {
	*whatsmeow.Client

	logger *slog.Logger
}

func NewClient(ctx context.Context, db *sql.DB, logger *slog.Logger) (*Client, error) {
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

	sendCtx, cancel := context.WithTimeout(ctx, sendMessageTimeout)
	defer cancel()

	_, err = c.SendMessage(sendCtx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})

	return err
}

func (c *Client) Download(msg whatsmeow.DownloadableMessage) ([]byte, error) {
	return c.Client.Download(context.Background(), msg)
}

func (c *Client) GetJID() types.JID {
	if c.Store.ID == nil {
		return types.JID{}
	}

	return *c.Store.ID
}

func LoginWithQR(client *whatsmeow.Client, logger *slog.Logger) error {
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

// whatsmeowLogger adapts our logger to whatsmeow's interface.
type whatsmeowLogger struct {
	logger *slog.Logger
}

func newWhatsmeowLogger(logger *slog.Logger) *whatsmeowLogger {
	return &whatsmeowLogger{logger: logger}
}

func (w *whatsmeowLogger) Errorf(msg string, args ...interface{}) {
	w.logger.Error(fmt.Sprintf(msg, args...))
}

func (w *whatsmeowLogger) Warnf(msg string, args ...interface{}) {
	w.logger.Warn(fmt.Sprintf(msg, args...))
}

func (w *whatsmeowLogger) Infof(msg string, args ...interface{}) {
	w.logger.Info(fmt.Sprintf(msg, args...))
}

func (w *whatsmeowLogger) Debugf(msg string, args ...interface{}) {
	w.logger.Debug(fmt.Sprintf(msg, args...))
}

func (w *whatsmeowLogger) Sub(module string) waLog.Logger {
	return &whatsmeowLogger{
		logger: w.logger.With("module", module),
	}
}
