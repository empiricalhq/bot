package service

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/internal/nameparser"
)

type ActionHandler interface {
	Execute(action string, state *domain.UserState, msg *message.Message, rawEvt interface{}, originatingNodeID string) error
}

type actionHandler struct {
	logger      *slog.Logger
	waClient    WhatsAppClient
	voucherPath string
}

func NewActionHandler(logger *slog.Logger, waClient WhatsAppClient, voucherPath string) ActionHandler {
	return &actionHandler{
		logger:      logger,
		waClient:    waClient,
		voucherPath: voucherPath,
	}
}

func (a *actionHandler) Execute(action string, state *domain.UserState, msg *message.Message, rawEvt interface{}, originatingNodeID string) error {
	if action == "" {
		return nil
	}

	a.logger.Debug("Executing action", "action", action, "user", state.UserID, "origin_node", originatingNodeID)

	switch action {
	case "save_user_name":
		return a.saveUserName(state, msg)
	case "clear_user_name":
		return a.clearUserName(state)
	case "set_selected_course":
		return a.setSelectedCourse(state, originatingNodeID)
	case "create_new_lead":
		a.logger.Info("New lead created", "user", state.UserID, "name", state.UserName)
	case "update_lead_interest_beginner":
		state.CourseInterest = "beginner"
	case "update_lead_interest_advanced":
		state.CourseInterest = "advanced"
	case "update_lead_consulted_price":
		state.ConsultedPrice = true
	case "escalate_to_human_agent":
		state.RequiresHumanAgent = true
		a.logger.Warn("Escalated to human", "user", state.UserID, "name", state.UserName)
	case "save_payment_voucher":
		return a.savePaymentVoucher(state, msg, rawEvt)
	default:
		a.logger.Warn("Unknown action", "action", action)

		return errors.New("unknown action: " + action)
	}

	return nil
}

// clearUserName deactivates name personalization by clearing the stored name
// with no stored name, the renderer falls back to the default.
func (a *actionHandler) clearUserName(state *domain.UserState) error {
	state.UserName = ""
	a.logger.Info("User name cleared by user request", "user", state.UserID)

	return nil
}

func (a *actionHandler) setSelectedCourse(state *domain.UserState, nodeID string) error {
	state.SelectedCourseID = nodeID
	a.logger.Info("User selected course for enrollment", "user", state.UserID, "course_node", nodeID)

	return nil
}

func (a *actionHandler) saveUserName(state *domain.UserState, msg *message.Message) error {
	nameInput := strings.TrimSpace(msg.Text)
	if nameInput == "" {
		return nil
	}

	finalNameToSave := nameInput
	stripKeywords := []string{"mi nombre es", "me llamo", "llámame"}

	lowerInput := strings.ToLower(nameInput)
	for _, keyword := range stripKeywords {
		if strings.HasPrefix(lowerInput, keyword) {
			// Strip the keyword prefix from the original string
			finalNameToSave = strings.TrimSpace(nameInput[len(keyword):])

			break
		}
	}

	if finalNameToSave == "" {
		a.logger.Warn("Name update resulted in empty string, ignoring", "user", state.UserID, "input", nameInput)

		return nil
	}

	if nameparser.Parse(finalNameToSave) == "" {
		a.logger.Warn("Invalid name input ignored during update attempt", "user", state.UserID, "input", finalNameToSave)

		return nil
	}

	state.UserName = finalNameToSave
	a.logger.Info("User name updated by user request", "user", state.UserID, "new_name", finalNameToSave)

	return nil
}

func (a *actionHandler) savePaymentVoucher(state *domain.UserState, msg *message.Message, rawEvt interface{}) error {
	msgEvent, ok := rawEvt.(*events.Message)
	if !ok {
		return errors.New("save_payment_voucher requires a raw message event")
	}

	imageMsg := msgEvent.Message.GetImageMessage()
	if imageMsg == nil {
		a.logger.Warn("save_payment_voucher triggered but message is not an image. Escalating.", "user", state.UserID, "media_type", msg.MediaType)
		state.RequiresHumanAgent = true

		return nil
	}

	data, err := a.waClient.Download(imageMsg)
	if err != nil {
		a.logger.Error("Failed to download voucher image", "error", err, "user", state.UserID)

		return fmt.Errorf("could not download voucher: %w", err)
	}

	if err := os.MkdirAll(a.voucherPath, 0o755); err != nil {
		a.logger.Error("Failed to create voucher directory", "error", err, "path", a.voucherPath)

		return fmt.Errorf("could not create voucher directory: %w", err)
	}

	// save the image
	filename := generateVoucherFilename(state.UserID, msg.PushName)
	filePath := filepath.Join(a.voucherPath, filename)

	err = os.WriteFile(filePath, data, 0o644)
	if err != nil {
		a.logger.Error("Failed to save voucher file", "error", err, "path", filePath)

		return fmt.Errorf("could not save voucher file: %w", err)
	}

	// update state with the path
	state.VoucherPath = filePath
	a.logger.Info("Payment voucher saved successfully", "user", state.UserID, "path", filePath)

	return nil
}

// Format: {phone_number}_{sanitized_push_name}_{timestamp}.jpeg.
func generateVoucherFilename(userID, pushName string) string {
	phone := userID
	if i := strings.Index(userID, "@"); i != -1 {
		phone = userID[:i]
	}

	sanitizedName := strings.ReplaceAll(pushName, " ", "_")
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	sanitizedName = reg.ReplaceAllString(sanitizedName, "")

	if sanitizedName == "" {
		sanitizedName = "user"
	}

	timestamp := time.Now().Unix()

	return fmt.Sprintf("%s_%s_%d.jpeg", phone, sanitizedName, timestamp)
}
