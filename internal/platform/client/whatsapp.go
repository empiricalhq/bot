package client

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"modernc.org/sqlite"

	"whatsbot/internal/logger"
)

func NewWhatsApp(ctx context.Context, db *sql.DB, logFactory *logger.Factory) (*whatsmeow.Client, error) {
	container := sqlstore.NewWithDB(db, "sqlite3", logger.NewWhatsmeowLogger(logFactory.GetLogger("sqlstore"), "sqlstore"))
	if err := container.Upgrade(ctx); err != nil {
		return nil, fmt.Errorf("failed to upgrade whatsmeow database schema: %w", err)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		// This is expected on first run. We log other errors as warnings.
		var sqliteErr *sqlite.Error
		if !errors.As(err, &sqliteErr) || sqliteErr.Code() != 1 { // 1 = SQLITE_ERROR
			logFactory.GetLogger("app").Warn("Could not get first device, may need QR login",
				map[string]interface{}{"error": err.Error()})
		}
	}

	whatsmeowLogger := logger.NewWhatsmeowLogger(logFactory.GetLogger("whatsmeow"), "whatsmeow")
	client := whatsmeow.NewClient(device, whatsmeowLogger)

	return client, nil
}

func LoginWithQR(client *whatsmeow.Client, log *logger.Logger, timeoutErr error) error {
	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get QR channel: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("connection failed during QR login: %w", err)
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		case "success":
			log.Info("QR login successful", nil)

			return nil
		case "timeout":
			return timeoutErr
		}
	}

	return errors.New("QR channel closed unexpectedly")
}
