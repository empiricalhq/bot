package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/internal/nameparser"
	"whatsbot/internal/repository"
	"whatsbot/internal/template"
)

const (
	messageTimeout                  = 30 * time.Second
	fallbackEscalationThreshold     = 3
	conversationTimeout             = 24 * time.Hour
	actionTriggerFallbackResponse   = "trigger_fallback_response"
	actionTriggerFallbackWrongMedia = "trigger_fallback_wrong_media"
)

var unsupportedMediaTypes = []string{"audio", "sticker", "video", "document"}

// OnMessageFunc defines the callback signature for message events.
type OnMessageFunc func(direction, userID, userName, text string)

// Bot orchestrates the WhatsBot runtime:
// - Handles incoming WA events
// - Manages user state (via FSM + repo)
// - Executes actions
// - Renders messages and sends responses
// - Applies safeguards (timeouts, ignored users, fallbacks).
type Bot struct {
	config    *config.Config
	repo      repository.Repository
	fsm       FSM
	actions   ActionHandler
	renderer  template.Renderer
	whatsapp  WhatsAppClient
	logger    *slog.Logger
	onMessage OnMessageFunc
}

// WhatsAppClient abstracts the minimal WA client methods
// so Bot can run against real or mocked implementations.
type WhatsAppClient interface {
	SendText(ctx context.Context, to, text string) error
	GetJID() types.JID
	Download(msg whatsmeow.DownloadableMessage) ([]byte, error)
}

func NewBot(
	config *config.Config,
	repo repository.Repository,
	fsm FSM,
	actions ActionHandler,
	renderer template.Renderer,
	whatsapp WhatsAppClient,
	logger *slog.Logger,
	onMessage OnMessageFunc,
) *Bot {
	return &Bot{
		config:    config,
		repo:      repo,
		fsm:       fsm,
		actions:   actions,
		renderer:  renderer,
		whatsapp:  whatsapp,
		logger:    logger,
		onMessage: onMessage,
	}
}

// HandleEvent is the entry point for all WhatsApp events.
// - Filters out non-messages and self-messages
// - Converts WA event => internal message
// - Retrieves or initializes user state
// - Routes existing users into FSM for processing.
func (b *Bot) HandleEvent(evt interface{}) {
	msgEvent, ok := evt.(*events.Message)
	if !ok {
		b.logger.Debug("Ignoring event: not a message", "event_type", fmt.Sprintf("%T", evt))

		return
	}

	b.logger.Debug("Received message event",
		"sender", msgEvent.Info.Sender.ToNonAD().String(),
		"is_from_me", msgEvent.Info.IsFromMe)

	// Ignore messages sent by this bot.
	// In PROD: ignore all self-messages.
	// In DEV: only allow self-messages from another linked device.
	if msgEvent.Info.IsFromMe {
		isDevMode := b.config.Environment == "dev"
		isFromAnotherDevice := msgEvent.Info.DeviceSentMeta != nil

		if !isDevMode || !isFromAnotherDevice {
			return
		}
	}

	msg := message.FromEvent(msgEvent)
	if msg == nil {
		b.logger.Debug("Ignoring event: message not processable", "sender", msgEvent.Info.Sender.ToNonAD().String())

		return
	}

	if b.shouldIgnoreUser(msg.SenderID) {
		b.logger.Debug("Ignoring user in dev mode", "user", msg.SenderID)

		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), messageTimeout)
	defer cancel()

	// Get user state. This step also handles onboarding new users.
	userState, isNewUser, err := b.getOrCreateUserState(ctx, msg)
	if err != nil {
		b.logger.Error("failed to get or create user state", "error", err, "user", msg.SenderID)

		return
	}

	// If the user is new, their onboarding is complete. Stop here.
	if isNewUser {
		b.logger.Info("new user onboarded and greeted", "user", msg.SenderID)

		return
	}

	// If the user already exists, process their message through the FSM.
	err = b.processExistingUserMessage(ctx, userState, msg, msgEvent)
	if err != nil {
		b.logger.Error("message processing failed for existing user", "error", err, "user", msg.SenderID)
	}
}

// processExistingUserMessage runs the FSM for a returning user:
// 1. Logs inbound message + invokes callback
// 2. Checks unsupported media (and replies if needed)
// 3. Determines next FSM node + action
// 4. Handles fallbacks (generic / wrong media / terminal nodes)
// 5. Executes actions (with error recovery + escalation)
// 6. Generates response, saves state, sends outbound message.
func (b *Bot) processExistingUserMessage(ctx context.Context, userState *domain.UserState, msg *message.Message, rawEvt interface{}) error {
	logger := b.logger.With("user", msg.SenderID, "from_node", userState.CurrentNode)
	logger.Debug("Processing message", "text", msg.Text, "has_media", msg.HasMedia)

	// Invoke the message callback for inbound messages.
	if b.onMessage != nil {
		b.onMessage("inbound", msg.SenderID, msg.PushName, msg.Text)
	}

	// Early exit for unsupported media types, unless the current node is expecting them.
	// This provides immediate, clear feedback to the user.
	if msg.HasMedia && slices.Contains(unsupportedMediaTypes, msg.MediaType) {
		currentNode := b.fsm.GetNode(userState.CurrentNode)
		canHandleMedia := false

		if currentNode != nil {
			for _, transition := range currentNode.Transitions {
				if transition.Condition.Type == "media_type" && slices.Contains(transition.Condition.Value, msg.MediaType) {
					canHandleMedia = true

					break
				}
			}
		}

		if !canHandleMedia {
			logger.Info("User sent an unsupported media type. Sending feedback.", "media_type", msg.MediaType)

			responseText := "Lo siento, no puedo procesar ese tipo de mensaje. Por favor, envíame un mensaje de texto. 😊"

			err := b.whatsapp.SendText(ctx, msg.SenderID, responseText)
			if err != nil {
				logger.Error("Failed to send unsupported media message", "error", err)
			}

			return nil
		}
	}

	originalNode := userState.CurrentNode

	var responseText string

	// 1: FSM decides the next node and action.
	nextNode, action := b.fsm.DetermineNext(userState, msg)

	isGenericFallback := (action == actionTriggerFallbackResponse)

	if !isGenericFallback {
		// Reset fallback counter when user makes valid progress or gets specific guidance.
		userState.RepromptCount = 0
	} else {
		userState.RepromptCount++
		logger.Debug("Generic fallback triggered", "count", userState.RepromptCount)

		if userState.RepromptCount >= fallbackEscalationThreshold {
			// Example: user keeps typing nonsense (e.g. '???') => escalate to human.
			logger.Warn("User stuck in fallback loop. Escalating to human agent.", "node", originalNode, "reprompt_count", userState.RepromptCount)

			nextNode = "NEEDS_ASSISTANCE"
			action = "escalate_to_human_agent"
			userState.RepromptCount = 0
		}
	}

	switch action {
	case actionTriggerFallbackWrongMedia:
		logger.Debug("Handling specific fallback for wrong media type", "node", originalNode)

		responseText = "Parece que enviaste un tipo de archivo incorrecto. Por favor, asegúrate de enviar una **imagen** (foto) para que pueda procesarlo. Gracias 😊"

	case actionTriggerFallbackResponse:
		fallbackNode := b.fsm.GetNode(originalNode)

		// A terminal node has a message but no transitions.
		// Example: "¡Ha sido un placer ayudarte!" => nothing else to offer.
		isTerminalNode := fallbackNode != nil && len(fallbackNode.Transitions) == 0 && fallbackNode.IncludeTransitions == ""

		if isTerminalNode {
			// On terminal nodes, a fallback means the conversation has likely ended (e.g., user says "thanks").
			// We remain silent to allow a natural pause. The user can re-engage with a global keyword.
			logger.Debug("Fallback on terminal node. No response sent.", "node", originalNode)

			responseText = ""
		} else if fallbackNode != nil {
			data := b.prepareTemplateData(userState)

			// Use custom fallback message if available.
			if fallbackNode.FallbackMessage != "" {
				logger.Debug("Fallback with custom message.", "node", originalNode)

				responseText = b.renderer.Render(fallbackNode.FallbackMessage, data)
			} else if fallbackNode.Message.Content != "" {
				// Otherwise, use the generic re-prompt for menu-like nodes.
				logger.Debug("Fallback on menu node. Re-prompting user.", "node", originalNode)

				fallbackPrefix := "No entendí tu respuesta 😊 Por favor, revisa las opciones:\n\n"
				responseText = b.renderer.Render(fallbackPrefix+fallbackNode.Message.Content, data)
			} else {
				logger.Warn("Fallback in a node with no message. Resetting to start.", "node", originalNode)

				nextNode = b.fsm.GetStartNode()
				responseText = b.generateResponse(nextNode, userState)
			}
		} else {
			logger.Warn("Fallback in a node with no message. Resetting to start.", "node", originalNode)

			nextNode = b.fsm.GetStartNode()
			responseText = b.generateResponse(nextNode, userState)
		}

	default:
		// Handles both normal transitions and escalations.
		logger.Debug("FSM determined next state", "to_node", nextNode, "action", action)

		if action != "" {
			err := b.actions.Execute(action, userState, msg, rawEvt, originalNode)
			if err != nil {
				logger.Error("Action failed", "action", action, "error", err)

				if errors.Is(err, ErrInvalidName) {
					responseText = "No pude reconocer eso como un nombre. ¿Podrías intentarlo de nuevo, por favor?"
					nextNode = originalNode // Stay in the current node to re-prompt
				} else {
					logger.Error("Critical action failure, escalating to human agent", "action", action, "error", err)

					responseText = "Hubo un problema al procesar tu comprobante. Por favor, contacta a una asesora para completar tu matrícula. Disculpa las molestias."
					nextNode = "NEEDS_ASSISTANCE"

					// Ensure the escalation action is executed to update state before saving.
					_ = b.actions.Execute("escalate_to_human_agent", userState, msg, rawEvt, originalNode)
				}
			}
		}

		if responseText == "" {
			responseText = b.generateResponse(nextNode, userState)
		}
	}

	// Update state.
	userState.CurrentNode = nextNode
	userState.LastUpdated = time.Now()

	inMsg := &domain.ConversationMessage{
		UserID:         msg.SenderID,
		Timestamp:      time.Now(),
		Direction:      "inbound",
		MessageContent: msg.Text,
		NodeID:         originalNode,
	}

	var outMsg *domain.ConversationMessage
	if responseText != "" {
		outMsg = &domain.ConversationMessage{
			UserID:         msg.SenderID,
			Timestamp:      time.Now(),
			Direction:      "outbound",
			MessageContent: responseText,
			NodeID:         nextNode,
		}
	}

	err := b.repo.SaveStateAndMessages(ctx, userState, inMsg, outMsg)
	if err != nil {
		return fmt.Errorf("failed to save state and messages: %w", err)
	}

	if responseText != "" {
		// Invoke the message callback for outbound messages.
		if b.onMessage != nil {
			b.onMessage("outbound", msg.SenderID, "Bot", responseText)
		}

		err := b.whatsapp.SendText(ctx, msg.SenderID, responseText)
		if err != nil {
			logger.Error("Failed to send message", "error", err)
		}
	}

	logger.Info("Message processed",
		"inbound_text", msg.Text,
		"outbound_text", responseText,
		"to_node", nextNode,
		"action", action,
	)

	return nil
}

// getOrCreateUserState ensures a user has a state in the repo:
// - If found and stale (>24h), resets to start node
// - If new, initializes state, sends greeting, persists
// Returns state + a flag (isNewUser).
func (b *Bot) getOrCreateUserState(ctx context.Context, msg *message.Message) (state *domain.UserState, isNew bool, err error) {
	userState, err := b.repo.GetUserState(ctx, msg.SenderID)
	if err != nil {
		return nil, false, fmt.Errorf("could not get user state from repository: %w", err)
	}

	// If CurrentNode is not empty, check if the conversation is stale
	if userState.CurrentNode != "" {
		isStale := time.Since(userState.LastUpdated) > conversationTimeout
		if isStale {
			b.logger.Info("Stale conversation detected. Resetting to start node.",
				"user", userState.UserID,
				"last_active", userState.LastUpdated,
				"previous_node", userState.CurrentNode,
			)

			userState.CurrentNode = b.fsm.GetStartNode()
			userState.RepromptCount = 0
		}
	}

	// If CurrentNode is empty (new user) or was reset (stale user), initialize them.
	if userState.CurrentNode == "" {
		b.logger.Info("New user initialized", "user", msg.SenderID, "node", b.fsm.GetStartNode(), "push_name", msg.PushName)

		userState.CurrentNode = b.fsm.GetStartNode()
		userState.UserName = msg.PushName
		userState.LastUpdated = time.Now()

		responseText := b.generateResponse(userState.CurrentNode, userState)
		if responseText == "" {
			b.logger.Warn("Start node has no message content, new user will not be greeted", "node", userState.CurrentNode)
		}

		outMsg := &domain.ConversationMessage{
			UserID:         msg.SenderID,
			Timestamp:      time.Now(),
			Direction:      "outbound",
			MessageContent: responseText,
			NodeID:         userState.CurrentNode,
		}

		err = b.repo.SaveStateAndMessages(ctx, userState, nil, outMsg)
		if err != nil {
			return nil, false, fmt.Errorf("failed to save new user state: %w", err)
		}

		if responseText != "" {
			// Invoke the message callback for the initial outbound message.
			if b.onMessage != nil {
				b.onMessage("outbound", msg.SenderID, "Bot", responseText)
			}

			err := b.whatsapp.SendText(ctx, msg.SenderID, responseText)
			if err != nil {
				b.logger.Error("Failed to send welcome message to new user", "error", err)
			}
		}

		return userState, true, nil
	}

	return userState, false, nil
}

// generateResponse fetches the FSM node's message content,
// fills it with template data, and returns the rendered text.
func (b *Bot) generateResponse(nodeID string, state *domain.UserState) string {
	node := b.fsm.GetNode(nodeID)
	if node == nil {
		b.logger.Error("Node not found", "node", nodeID)

		return "Sorry, something went wrong."
	}

	if node.Message.Content == "" {
		b.logger.Debug("Node has no message content", "node", nodeID)

		return ""
	}

	data := b.prepareTemplateData(state)

	return b.renderer.Render(node.Message.Content, data)
}

// prepareTemplateData builds a data map for template rendering:
// - Normalizes user's name via the nameparser
// - Inserts dynamic greeting (welcome vs returning)
// - Injects selected course name if available.
func (b *Bot) prepareTemplateData(state *domain.UserState) map[string]string {
	data := make(map[string]string)

	// Set user name
	name := nameparser.Parse(state.UserName)
	if name == "" {
		name = "amigx"
	}

	data["name"] = name

	// Dynamic greeting:
	// - New users (0–2 messages so far) => show a welcome.
	//   Example: just joined and sent first message.
	// - Returning users (>2 messages) => show a "welcome back".
	msgCount, err := b.repo.GetUserMessageCount(context.Background(), state.UserID)
	if err != nil {
		b.logger.Error("Failed to get user message count for dynamic greeting", "error", err, "user", state.UserID)
		// Fallback: assume returning user.
		data["greeting"] = "Qué gusto verte de nuevo."
	} else if msgCount <= 2 {
		data["greeting"] = "¡Bienvenidx! Es un placer ayudarte a empezar."
	} else {
		data["greeting"] = "Qué gusto verte de nuevo."
	}

	if state.SelectedCourseID != "" {
		courseNode := b.fsm.GetNode(state.SelectedCourseID)
		if courseNode != nil && courseNode.Title != "" {
			data["course_name"] = courseNode.Title
		} else {
			data["course_name"] = "el curso seleccionado"

			b.logger.Warn("Could not find title for selected course", "user", state.UserID, "course_id", state.SelectedCourseID)
		}
	}

	return data
}

// shouldIgnoreUser determines if a user should be ignored
// (dev mode only, unless whitelisted via DEV_ALLOWED_USERS).
func (b *Bot) shouldIgnoreUser(userID string) bool {
	if b.config.Environment != "dev" {
		return false
	}

	if len(b.config.DevAllowedUsers) == 0 {
		b.logger.Debug("There are no allowed users set. Add DEV_ALLOWED_USERS for testing.")

		return true // In dev mode with no allowed users, ignore all
	}

	_, allowed := b.config.DevAllowedUsers[userID]

	return !allowed
}
