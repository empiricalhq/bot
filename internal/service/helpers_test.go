//nolint:goconst // Flow node IDs repeat across helpers and cases; literals keep them readable against conversation.json.
package service_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/internal/service"
	"whatsbot/internal/template"
)

const (
	testPhone  = "51999000111"
	testUserID = testPhone + "@s.whatsapp.net"
	testName   = "Ana Pérez"
)

// The bot sleeps for TYPING_DELAY_MS before every send. Tests must never wait.
func TestMain(m *testing.M) {
	err := os.Unsetenv("TYPING_DELAY_MS")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// fakeRepo is an in-memory Repository. Like the SQL repository it hands out
// copies, returns an empty state for unknown users, and counts stored messages.
type fakeRepo struct {
	states   map[string]domain.UserState
	messages []domain.ConversationMessage
	saves    int

	getStateErr error
	saveErr     error
	countErr    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{states: make(map[string]domain.UserState)}
}

func (r *fakeRepo) GetUserState(_ context.Context, userID string) (*domain.UserState, error) {
	if r.getStateErr != nil {
		return nil, r.getStateErr
	}

	state := r.states[userID]
	state.UserID = userID

	return &state, nil
}

func (r *fakeRepo) SaveStateAndMessages(_ context.Context, state *domain.UserState, inMsg, outMsg *domain.ConversationMessage) error {
	if r.saveErr != nil {
		return r.saveErr
	}

	r.saves++
	r.states[state.UserID] = *state

	if inMsg != nil {
		r.messages = append(r.messages, *inMsg)
	}

	if outMsg != nil {
		r.messages = append(r.messages, *outMsg)
	}

	return nil
}

func (r *fakeRepo) InitSchema(context.Context) error {
	return nil
}

func (r *fakeRepo) GetUserMessageCount(_ context.Context, userID string) (int, error) {
	if r.countErr != nil {
		return 0, r.countErr
	}

	count := 0

	for _, stored := range r.messages {
		if stored.UserID == userID {
			count++
		}
	}

	return count, nil
}

func (r *fakeRepo) seed(state domain.UserState) {
	r.states[state.UserID] = state
}

func (r *fakeRepo) state() domain.UserState {
	return r.states[testUserID]
}

// fakeWA records every SendText attempt, including failed ones.
type fakeWA struct {
	sent        []sentText
	sendErr     error
	data        []byte
	downloadErr error
	downloads   int
}

type sentText struct {
	to, text string
}

func (w *fakeWA) SendText(_ context.Context, to, text string) error {
	w.sent = append(w.sent, sentText{to: to, text: text})

	return w.sendErr
}

func (w *fakeWA) GetJID() types.JID {
	return types.JID{}
}

func (w *fakeWA) Download(whatsmeow.DownloadableMessage) ([]byte, error) {
	w.downloads++

	return w.data, w.downloadErr
}

type callbackCall struct {
	direction, userID, userName, text string
}

type callbackRecorder struct {
	calls []callbackCall
}

func (c *callbackRecorder) record(direction, userID, userName, text string) {
	c.calls = append(c.calls, callbackCall{direction, userID, userName, text})
}

// harness wires a real FSM, renderer and action handler to in-memory fakes.
type harness struct {
	bot        *service.Bot
	repo       *fakeRepo
	wa         *fakeWA
	calls      *callbackRecorder
	flow       *domain.Flow
	voucherDir string
}

type harnessConfig struct {
	flow       *domain.Flow
	cfg        *config.Config
	fsm        service.FSM
	noCallback bool
}

type harnessOption func(*harnessConfig)

func withFlow(flow *domain.Flow) harnessOption {
	return func(settings *harnessConfig) { settings.flow = flow }
}

func withConfig(cfg *config.Config) harnessOption {
	return func(settings *harnessConfig) { settings.cfg = cfg }
}

func withFSM(fsm service.FSM) harnessOption {
	return func(settings *harnessConfig) { settings.fsm = fsm }
}

func withoutCallback() harnessOption {
	return func(settings *harnessConfig) { settings.noCallback = true }
}

func loadRealFlow(t *testing.T) *domain.Flow {
	t.Helper()

	flow, err := service.LoadFlow("../../conversation.json")
	if err != nil {
		t.Fatalf("loading conversation.json: %v", err)
	}

	return flow
}

func newHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()

	settings := harnessConfig{cfg: &config.Config{Environment: "prod"}}
	for _, opt := range opts {
		opt(&settings)
	}

	if settings.flow == nil {
		settings.flow = loadRealFlow(t)
	}

	logger := discardLogger()

	fsm := settings.fsm
	if fsm == nil {
		fsm = service.NewFSM(settings.flow, logger)
	}

	harn := &harness{
		repo:       newFakeRepo(),
		wa:         &fakeWA{data: []byte("jpeg-bytes")},
		calls:      &callbackRecorder{},
		flow:       settings.flow,
		voucherDir: filepath.Join(t.TempDir(), "vouchers"),
	}

	actions := service.NewActionHandler(logger, harn.wa, harn.voucherDir)
	onMessage := harn.calls.record
	if settings.noCallback {
		onMessage = nil
	}

	harn.bot = service.NewBot(settings.cfg, harn.repo, fsm, actions, template.NewRenderer(logger), harn.wa, logger, onMessage)

	return harn
}

// send delivers a plain text message from the test user.
func (h *harness) send(text string) {
	h.bot.HandleEvent(textEvent(text))
}

// seed stores an existing, recently active user at the given node.
func (h *harness) seed(node string) {
	h.repo.seed(domain.UserState{UserID: testUserID, CurrentNode: node, UserName: "Ana", LastUpdated: time.Now()})
}

// nodeText renders a node's message the way the bot does, for the given template data.
func (h *harness) nodeText(nodeID string, data map[string]string) string {
	return template.NewRenderer(discardLogger()).Render(h.flow.Nodes[nodeID].Message.Content, data)
}

func (h *harness) lastSent(t *testing.T) string {
	t.Helper()

	if len(h.wa.sent) == 0 {
		t.Fatal("nothing was sent")
	}

	return h.wa.sent[len(h.wa.sent)-1].text
}

func newEvent(content *waE2E.Message) *events.Message {
	jid := types.NewJID(testPhone, types.DefaultUserServer)

	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: jid, Sender: jid},
			PushName:      testName,
		},
		Message: content,
	}
}

func textEvent(text string) *events.Message {
	return newEvent(&waE2E.Message{Conversation: &text})
}

func extendedText(text string) *waE2E.Message {
	return &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: &text}}
}

// emptyEvent carries a payload the bot cannot use, such as a reaction.
func emptyEvent() *events.Message {
	return newEvent(&waE2E.Message{})
}

func imageEvent(caption string) *events.Message {
	return newEvent(&waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: &caption}})
}

// mediaEvent builds an audio, sticker, video or document message.
func mediaEvent(kind, caption string) *events.Message {
	content := &waE2E.Message{}

	switch kind {
	case "audio":
		content.AudioMessage = &waE2E.AudioMessage{}
	case "sticker":
		content.StickerMessage = &waE2E.StickerMessage{}
	case "video":
		content.VideoMessage = &waE2E.VideoMessage{Caption: &caption}
	case "document":
		content.DocumentMessage = &waE2E.DocumentMessage{Caption: &caption}
	default:
		panic("unknown media kind " + kind)
	}

	return newEvent(content)
}

func textMsg(text string) message.Message {
	return message.Message{Text: text, SenderID: testUserID}
}

func mediaMsg(mediaType, caption string) message.Message {
	return message.Message{Text: caption, SenderID: testUserID, HasMedia: true, MediaType: mediaType}
}

//nolint:ireturn // service.NewFSM itself returns the interface; the tests use it only through that contract.
func newTestFSM(flow *domain.Flow) service.FSM {
	return service.NewFSM(flow, discardLogger())
}

// Condition and transition builders for inline flows.

func keyword(values ...string) domain.Condition {
	return domain.Condition{Type: "keyword", Value: values}
}

func exact(values ...string) domain.Condition {
	return domain.Condition{Type: "exact", Value: values}
}

func regex(pattern string) domain.Condition {
	return domain.Condition{Type: "regex", Regex: pattern}
}

func mediaType(values ...string) domain.Condition {
	return domain.Condition{Type: "media_type", Value: values}
}

func anyText() domain.Condition {
	return domain.Condition{Type: "any_text"}
}

func anyMedia() domain.Condition {
	return domain.Condition{Type: "media"}
}

func goTo(cond domain.Condition, target, action string) domain.Transition {
	return domain.Transition{Condition: cond, Target: target, Action: action}
}

func textNode(content string, transitions ...domain.Transition) domain.Node {
	return domain.Node{Message: domain.MessageContent{Type: "text", Content: content}, Transitions: transitions}
}

func flowOf(start string, nodes map[string]domain.Node, globals ...domain.Transition) *domain.Flow {
	return &domain.Flow{StartNode: start, Nodes: nodes, GlobalTransitions: globals}
}

// Texts that bot.go hardcodes.
const (
	welcomeGreeting    = "¡Bienvenidx! Es un placer ayudarte a empezar."
	returningGreeting  = "Qué gusto verte de nuevo."
	unsupportedMedia   = "Lo siento, no puedo procesar ese tipo de mensaje. Por favor, envíame un mensaje de texto. 😊"
	wrongMediaText     = "Parece que enviaste un tipo de archivo incorrecto. Por favor, asegúrate de enviar una **imagen** (foto) para que pueda procesarlo. Gracias 😊"
	invalidNameText    = "No pude reconocer eso como un nombre. ¿Podrías intentarlo de nuevo, por favor?"
	voucherFailureText = "Hubo un problema al procesar tu comprobante. Por favor, contacta a una asesora para completar tu matrícula. Disculpa las molestias."
	menuFallbackPrefix = "No entendí tu respuesta 😊 Por favor, revisa las opciones:\n\n"
)

func templateData(name, greeting string) map[string]string {
	return map[string]string{"name": name, "greeting": greeting}
}

func (h *harness) render(content string, data map[string]string) string {
	return template.NewRenderer(discardLogger()).Render(content, data)
}

// storedMessage is a ConversationMessage without the fields that vary between runs.
type storedMessage struct {
	direction, nodeID, content string
}

func (h *harness) transcript() []storedMessage {
	stored := make([]storedMessage, 0, len(h.repo.messages))
	for _, msg := range h.repo.messages {
		stored = append(stored, storedMessage{msg.Direction, msg.NodeID, msg.MessageContent})
	}

	return stored
}

// sentTexts lists the text of every send attempt, checking each went to the test user.
func (h *harness) sentTexts(t *testing.T) []string {
	t.Helper()

	texts := make([]string, 0, len(h.wa.sent))

	for _, sent := range h.wa.sent {
		if sent.to != testUserID {
			t.Errorf("message sent to %q, want %q", sent.to, testUserID)
		}

		texts = append(texts, sent.text)
	}

	return texts
}

func (h *harness) wantSent(t *testing.T, want ...string) {
	t.Helper()

	got := h.sentTexts(t)
	if !slices.Equal(got, want) {
		t.Errorf("sent texts = %q, want %q", got, want)
	}
}

func (h *harness) wantNode(t *testing.T, want string) {
	t.Helper()

	if got := h.repo.state().CurrentNode; got != want {
		t.Errorf("stored node = %q, want %q", got, want)
	}
}

func (h *harness) wantTranscript(t *testing.T, want ...storedMessage) {
	t.Helper()

	if got := h.transcript(); !slices.Equal(got, want) {
		t.Errorf("stored messages = %+v, want %+v", got, want)
	}
}
