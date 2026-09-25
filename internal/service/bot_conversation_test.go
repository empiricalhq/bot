//nolint:goconst // Flow node IDs repeat across cases; literals keep each case readable against conversation.json.
package service_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
)

func TestTextReplyAdvancesTheFlow(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.repo.seed(domain.UserState{
		UserID:        testUserID,
		CurrentNode:   "MAIN_MENU",
		UserName:      "Ana",
		RepromptCount: 2,
		LastUpdated:   time.Now().Add(-time.Hour),
	})

	harn.send("precio")

	want := harn.nodeText("CONSULTED_PRICE", templateData("Ana", welcomeGreeting))
	harn.wantSent(t, want)
	harn.wantTranscript(t,
		storedMessage{"inbound", "MAIN_MENU", "precio"},
		storedMessage{"outbound", "CONSULTED_PRICE", want},
	)

	state := harn.repo.state()
	if state.CurrentNode != "CONSULTED_PRICE" || state.RepromptCount != 0 || state.UserName != "Ana" {
		t.Errorf("state = %+v, want CONSULTED_PRICE with the reprompt count reset", state)
	}

	if time.Since(state.LastUpdated) > time.Minute {
		t.Errorf("LastUpdated = %v, want now", state.LastUpdated)
	}

	wantCalls := []callbackCall{
		{"inbound", testUserID, testName, "precio"},
		{"outbound", testUserID, "Bot", want},
	}
	if len(harn.calls.calls) != 2 || harn.calls.calls[0] != wantCalls[0] || harn.calls.calls[1] != wantCalls[1] {
		t.Errorf("callbacks = %+v, want %+v", harn.calls.calls, wantCalls)
	}
}

func TestGreetingDependsOnTheStoredMessageCount(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		stored   int
		countErr error
		want     string
	}{
		{"no messages yet", 0, nil, welcomeGreeting},
		{"two messages", 2, nil, welcomeGreeting},
		{"three messages", 3, nil, returningGreeting},
		{"count unavailable", 0, errors.New("db down"), returningGreeting},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t)
			harn.seed("GREETING_INTRO")
			harn.repo.countErr = testCase.countErr

			for range testCase.stored {
				harn.repo.messages = append(harn.repo.messages, domain.ConversationMessage{UserID: testUserID})
			}

			harn.send("menu")

			harn.wantSent(t, harn.nodeText("MAIN_MENU", templateData("Ana", testCase.want)))
		})
	}
}

func TestCourseNameInTemplates(t *testing.T) {
	t.Parallel()

	t.Run("selected through the flow", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("COURSE_CORAZON")

		harn.send("quiero inscribirme")

		data := templateData("Ana", welcomeGreeting)
		data["course_name"] = "CURSO JOYAS MODELO CORAZÓN"
		harn.wantSent(t, harn.nodeText("CONFIRM_ENROLLMENT", data))

		if got := harn.repo.state().SelectedCourseID; got != "COURSE_CORAZON" {
			t.Errorf("SelectedCourseID = %q, want COURSE_CORAZON", got)
		}
	})

	t.Run("selected node has no title", func(t *testing.T) {
		t.Parallel()

		for _, selected := range []string{"MAIN_MENU", "NO_SUCH_NODE"} {
			harn := newHarness(t)
			harn.repo.seed(domain.UserState{
				UserID: testUserID, CurrentNode: "CONFIRM_ENROLLMENT", UserName: "Ana", SelectedCourseID: selected, LastUpdated: time.Now(),
			})

			harn.send("zzzz")

			data := templateData("Ana", welcomeGreeting)
			data["course_name"] = "el curso seleccionado"
			harn.wantSent(t, harn.render(menuFallbackPrefix+harn.flow.Nodes["CONFIRM_ENROLLMENT"].Message.Content, data))
		}
	})
}

func TestUnsupportedMedia(t *testing.T) {
	t.Parallel()

	kinds := []string{"audio", "sticker", "video", "document"}

	for _, node := range []string{"MAIN_MENU", "ENROLLMENT_PROCESS"} {
		for _, kind := range kinds {
			t.Run(node+" "+kind, func(t *testing.T) {
				t.Parallel()

				harn := newHarness(t)
				harn.repo.seed(domain.UserState{UserID: testUserID, CurrentNode: node, UserName: "Ana", RepromptCount: 1, LastUpdated: time.Now()})
				before := harn.repo.state()

				harn.bot.HandleEvent(mediaEvent(kind, "mira esto"))

				harn.wantSent(t, unsupportedMedia)

				// Doubt: the exchange is neither stored nor counted; the state is not even touched.
				if harn.repo.saves != 0 || harn.repo.state() != before || len(harn.repo.messages) != 0 {
					t.Errorf("saves = %d, state = %+v; want nothing persisted", harn.repo.saves, harn.repo.state())
				}

				// Doubt: only the inbound callback fires, so a UI never shows the reply.
				if len(harn.calls.calls) != 1 || harn.calls.calls[0].direction != "inbound" {
					t.Errorf("callbacks = %+v, want only the inbound one", harn.calls.calls)
				}
			})
		}
	}
}

func TestUnsupportedMediaReplyFailureIsIgnored(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")
	harn.wa.sendErr = errors.New("offline")

	harn.bot.HandleEvent(mediaEvent("audio", ""))

	if len(harn.wa.sent) != 1 || harn.repo.saves != 0 {
		t.Errorf("sent %d, saves %d; want one failed attempt and no save", len(harn.wa.sent), harn.repo.saves)
	}
}

func TestUnsupportedMediaOnANodeThatDoesNotExist(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("GHOST")

	harn.bot.HandleEvent(mediaEvent("sticker", ""))

	harn.wantSent(t, unsupportedMedia)
	harn.wantNode(t, "GHOST")

	// A text message resets the user to the start node and shows the start message.
	harn.send("hola")

	harn.wantNode(t, "GREETING_INTRO")
	harn.wantSent(t, unsupportedMedia, harn.nodeText("GREETING_INTRO", templateData("Ana", welcomeGreeting)))
}

func mediaFlow() *domain.Flow {
	return flowOf("START", map[string]domain.Node{
		"START":       textNode("start"),
		"WANTS_VIDEO": textNode("send a video", goTo(mediaType("video"), "DONE", "")),
		"WANTS_ANY":   textNode("send anything", goTo(anyMedia(), "DONE", "")),
		"DONE":        textNode("thanks"),

		"NEEDS_ASSISTANCE": textNode("a person will help you"),
	})
}

func TestMediaTheNodeAsksForIsAccepted(t *testing.T) {
	t.Parallel()

	harn := newHarness(t, withFlow(mediaFlow()))
	harn.seed("WANTS_VIDEO")

	harn.bot.HandleEvent(mediaEvent("video", ""))

	harn.wantSent(t, "thanks")
	harn.wantNode(t, "DONE")
}

func TestMediaConditionDoesNotLiftTheUnsupportedMediaCheck(t *testing.T) {
	t.Parallel()

	harn := newHarness(t, withFlow(mediaFlow()))
	harn.seed("WANTS_ANY")

	harn.bot.HandleEvent(mediaEvent("sticker", ""))

	// Doubt: only a media_type transition lets unsupported media through, so a node with a
	// "media" (any media) transition never receives a sticker, audio, video or document.
	harn.wantSent(t, unsupportedMedia)
	harn.wantNode(t, "WANTS_ANY")

	harn.bot.HandleEvent(imageEvent(""))

	harn.wantNode(t, "DONE")
}

func TestWrongMediaType(t *testing.T) {
	t.Parallel()

	harn := newHarness(t, withFlow(mediaFlow()))
	harn.seed("WANTS_VIDEO")

	harn.bot.HandleEvent(imageEvent(""))

	reply := wrongMediaReply("un **video**")
	harn.wantSent(t, reply)
	harn.wantTranscript(t,
		storedMessage{"inbound", "WANTS_VIDEO", ""},
		storedMessage{"outbound", "WANTS_VIDEO", reply},
	)

	// Wrong files count like wrong text: the third one hands the user to a human.
	harn.bot.HandleEvent(imageEvent(""))

	state := harn.repo.state()
	if state.CurrentNode != "WANTS_VIDEO" || state.RepromptCount != 2 || state.RequiresHumanAgent {
		t.Errorf("state after two wrong files = %+v, want the user still waiting with reprompt count 2", state)
	}

	harn.bot.HandleEvent(imageEvent(""))

	state = harn.repo.state()
	if state.CurrentNode != "NEEDS_ASSISTANCE" || !state.RequiresHumanAgent || state.RepromptCount != 0 {
		t.Errorf("state after three wrong files = %+v, want escalation with the count reset", state)
	}

	if got := harn.lastSent(t); got != "a person will help you" {
		t.Errorf("last message = %q, want the assistance node", got)
	}
}

func TestWrongMediaReplyNamesEveryExpectedType(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{
		"START":          textNode("start"),
		"WANTS_DOCUMENT": textNode("send a file", goTo(mediaType("document"), "START", "")),
		"WANTS_EITHER":   textNode("send a video or a file", goTo(mediaType("video", "document"), "START", "")),
		"WANTS_AUDIO":    textNode("send a voice note", goTo(mediaType("audio"), "START", "")),
		"WANTS_STICKER":  textNode("send a sticker", goTo(mediaType("sticker"), "START", "")),
	})

	cases := []struct {
		node string
		want string
	}{
		{"WANTS_DOCUMENT", "un **documento**"},
		{"WANTS_EITHER", "un **video** o un **documento**"},
		{"WANTS_AUDIO", "un **audio**"},
		{"WANTS_STICKER", "un **sticker**"},
	}

	for _, testCase := range cases {
		t.Run(testCase.node, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t, withFlow(flow))
			harn.seed(testCase.node)

			// An image is the one file type that reaches the flow of a node that asks for something else.
			harn.bot.HandleEvent(imageEvent(""))

			harn.wantSent(t, wrongMediaReply(testCase.want))
		})
	}
}

func TestGlobalKeywords(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		node string
	}{
		{"adios", "CONVERSATION_CLOSED"},
		{"hasta luego", "CONVERSATION_CLOSED"},
		{"cambiar nombre", "CHANGE_NAME_PROMPT"},
		{"quiero hablar con un humano", "NEEDS_ASSISTANCE"},
		{"volver al inicio", "MAIN_MENU"},
	}

	for _, testCase := range cases {
		t.Run(testCase.text, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t)
			harn.seed("CONSULTED_SCHEDULE")

			harn.send(testCase.text)

			harn.wantNode(t, testCase.node)
			harn.wantSent(t, harn.nodeText(testCase.node, templateData("Ana", welcomeGreeting)))
		})
	}
}

func TestHelpEscalatesOnEntry(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")

	harn.send("ayuda")

	harn.wantNode(t, "NEEDS_ASSISTANCE")

	if !harn.repo.state().RequiresHumanAgent {
		t.Error("RequiresHumanAgent is not set on entering the help node")
	}
}

func TestLeadFieldsAreRecordedOnEnteringTheNode(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("GREETING_INTRO")

	harn.send("1")
	harn.wantNode(t, "INTERESTED_IN_BEGINNER")

	if got := harn.repo.state().CourseInterest; got != "beginner" {
		t.Errorf("CourseInterest = %q after entering the node, want beginner", got)
	}

	// Leaving by a global keyword must not matter: the interest was recorded when the node was entered.
	harn.send("adios")
	harn.wantNode(t, "CONVERSATION_CLOSED")

	if got := harn.repo.state().CourseInterest; got != "beginner" {
		t.Errorf("CourseInterest = %q after a global exit, want beginner", got)
	}

	harn.repo.seed(domain.UserState{UserID: testUserID, CurrentNode: "MAIN_MENU", LastUpdated: time.Now()})

	harn.send("precio")
	harn.wantNode(t, "CONSULTED_PRICE")

	if !harn.repo.state().ConsultedPrice {
		t.Error("ConsultedPrice is not set on entering the price node")
	}
}

func TestNodeActionRunsAfterTheTransitionAction(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("COURSE_CORAZON")

	harn.send("quiero inscribirme")

	// The transition records the course being left; the entered node has no action of its own.
	harn.wantNode(t, "CONFIRM_ENROLLMENT")

	if got := harn.repo.state().SelectedCourseID; got != "COURSE_CORAZON" {
		t.Errorf("SelectedCourseID = %q, want COURSE_CORAZON", got)
	}

	// The confirmation node selects itself as the course when entered.
	harn.seed("CONSULTED_PRICE")

	harn.send("matricula")

	harn.wantNode(t, "CONFIRM_ENROLLMENT_BEGINNER")

	if got := harn.repo.state().SelectedCourseID; got != "CONFIRM_ENROLLMENT_BEGINNER" {
		t.Errorf("SelectedCourseID = %q, want CONFIRM_ENROLLMENT_BEGINNER", got)
	}
}

func TestGreetingDoesNotSelectAMenuOption(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")

	harn.send("hola")

	harn.wantNode(t, "MAIN_MENU")
	harn.wantSent(t, harn.render(menuFallbackPrefix+harn.flow.Nodes["MAIN_MENU"].Message.Content, templateData("Ana", welcomeGreeting)))
}

func TestNameChanges(t *testing.T) {
	t.Parallel()

	t.Run("prompt then a new name", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		harn.send("cambiar nombre")
		harn.send("Lucía Gómez")

		state := harn.repo.state()
		if state.CurrentNode != "NAME_CHANGE_CONFIRMED" || state.UserName != "Lucía Gómez" {
			t.Errorf("state = %+v, want NAME_CHANGE_CONFIRMED with the new name", state)
		}

		if got := harn.lastSent(t); got != harn.nodeText("NAME_CHANGE_CONFIRMED", templateData("Lucía", welcomeGreeting)) {
			t.Errorf("last message = %q", got)
		}
	})

	t.Run("announced in one message", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		harn.send("Mi nombre es Carla")

		harn.wantNode(t, "NAME_CHANGE_CONFIRMED")

		if got := harn.repo.state().UserName; got != "Carla" {
			t.Errorf("UserName = %q, want Carla", got)
		}
	})

	t.Run("name removed", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		harn.send("sin nombre")

		harn.wantNode(t, "NAME_REMOVED_CONFIRMATION")
		harn.wantSent(t, harn.nodeText("NAME_REMOVED_CONFIRMATION", templateData("amigx", welcomeGreeting)))

		if got := harn.repo.state().UserName; got != "" {
			t.Errorf("UserName = %q, want it cleared", got)
		}
	})
}

func TestNameChangeEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("a sentence is stored whole and greets with its first word", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		// Doubt: see the save_user_name action test; the user is now greeted as "Hola".
		harn.send("hola, me llamo Ana")

		if got := harn.repo.state().UserName; got != "hola, me llamo Ana" {
			t.Errorf("UserName = %q, want the whole sentence", got)
		}

		harn.wantSent(t, harn.nodeText("NAME_CHANGE_CONFIRMED", templateData("Hola", welcomeGreeting)))
	})

	t.Run("bare prefix keeps the name but confirms the change", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		// Doubt: "He actualizado tu nombre" is sent although the name did not change.
		harn.send("mi nombre es")

		harn.wantNode(t, "NAME_CHANGE_CONFIRMED")

		if got := harn.repo.state().UserName; got != "Ana" {
			t.Errorf("UserName = %q, want it unchanged", got)
		}
	})

	t.Run("invalid name at the prompt", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("CHANGE_NAME_PROMPT")

		harn.send("xyz")

		harn.wantSent(t, invalidNameText)
		harn.wantNode(t, "CHANGE_NAME_PROMPT")
		harn.wantTranscript(t,
			storedMessage{"inbound", "CHANGE_NAME_PROMPT", "xyz"},
			storedMessage{"outbound", "CHANGE_NAME_PROMPT", invalidNameText},
		)

		if got := harn.repo.state().UserName; got != "Ana" {
			t.Errorf("UserName = %q, want it unchanged", got)
		}
	})

	t.Run("invalid name announced from another node", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("MAIN_MENU")

		harn.send("mi nombre es xyz")

		harn.wantSent(t, invalidNameText)
		harn.wantNode(t, "MAIN_MENU")
	})

	t.Run("help typed at the name prompt becomes the name", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.seed("CHANGE_NAME_PROMPT")

		// Doubt: any_text catches "ayuda" before the help override can.
		harn.send("ayuda")

		harn.wantNode(t, "NAME_CHANGE_CONFIRMED")

		if got := harn.repo.state().UserName; got != "ayuda" {
			t.Errorf("UserName = %q, want ayuda", got)
		}
	})
}

func TestFallbackOnAMenuNode(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")

	menu := harn.flow.Nodes["MAIN_MENU"].Message.Content
	reprompt := harn.render(menuFallbackPrefix+menu, templateData("Ana", welcomeGreeting))

	harn.send("zzzz")
	harn.wantSent(t, reprompt)
	harn.wantNode(t, "MAIN_MENU")

	if got := harn.repo.state().RepromptCount; got != 1 {
		t.Errorf("RepromptCount = %d, want 1", got)
	}

	harn.wantTranscript(t,
		storedMessage{"inbound", "MAIN_MENU", "zzzz"},
		storedMessage{"outbound", "MAIN_MENU", reprompt},
	)

	// A valid answer resets the count.
	harn.send("horarios")
	harn.wantNode(t, "CONSULTED_SCHEDULE")

	if got := harn.repo.state().RepromptCount; got != 0 {
		t.Errorf("RepromptCount = %d after progress, want 0", got)
	}
}

func TestThirdFallbackEscalatesToAHuman(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")

	harn.send("zzzz")
	harn.send("zzzz")

	if state := harn.repo.state(); state.RepromptCount != 2 || state.RequiresHumanAgent || state.CurrentNode != "MAIN_MENU" {
		t.Fatalf("state after two fallbacks = %+v", state)
	}

	harn.send("zzzz")

	state := harn.repo.state()
	if state.CurrentNode != "NEEDS_ASSISTANCE" || !state.RequiresHumanAgent || state.RepromptCount != 0 {
		t.Errorf("state after the third fallback = %+v, want escalation with the count reset", state)
	}

	if got := harn.lastSent(t); got != harn.nodeText("NEEDS_ASSISTANCE", templateData("Ana", returningGreeting)) {
		t.Errorf("last message = %q, want the assistance node", got)
	}
}

func TestFallbackOnTheTerminalNodeIsSilent(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("URGENT_ASSISTANCE")

	harn.send("gracias")

	harn.wantSent(t)
	harn.wantNode(t, "URGENT_ASSISTANCE")
	// Only the inbound message is stored, and the callback fires only for it.
	harn.wantTranscript(t, storedMessage{"inbound", "URGENT_ASSISTANCE", "gracias"})

	if len(harn.calls.calls) != 1 || harn.repo.state().RepromptCount != 1 {
		t.Errorf("callbacks = %+v, RepromptCount = %d", harn.calls.calls, harn.repo.state().RepromptCount)
	}

	// Doubt: silence still counts as a fallback, so a third "thanks" pulls the user out of the
	// terminal node into a fresh escalation.
	harn.send("gracias")
	harn.send("gracias")

	harn.wantNode(t, "NEEDS_ASSISTANCE")
	harn.wantSent(t, harn.nodeText("NEEDS_ASSISTANCE", templateData("Ana", welcomeGreeting)))
}

func TestFallbackUsesTheNodesCustomMessage(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{
		"START": textNode("start"),
		"ASK": {
			Message:         domain.MessageContent{Content: "pick"},
			FallbackMessage: "Lo siento {{name}}, intenta de nuevo",
			Transitions:     []domain.Transition{goTo(exact("ok"), "START", "")},
		},
		// A node with a fallback message but no message of its own.
		"SILENT_ASK": {
			FallbackMessage: "custom only",
			Transitions:     []domain.Transition{goTo(exact("ok"), "START", "")},
		},
	})
	harn := newHarness(t, withFlow(flow))

	harn.seed("ASK")
	harn.send("nope")
	harn.wantSent(t, "Lo siento Ana, intenta de nuevo")

	harn.seed("SILENT_ASK")
	harn.send("nope")
	harn.wantSent(t, "Lo siento Ana, intenta de nuevo", "custom only")
}

func TestFallbackOnANodeWithoutAnyMessageResetsToStart(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{
		"START": textNode("welcome"),
		"BLANK": textNode("", goTo(exact("ok"), "START", "")),
	})
	harn := newHarness(t, withFlow(flow))
	harn.seed("BLANK")

	harn.send("nope")

	harn.wantSent(t, "welcome")
	harn.wantNode(t, "START")

	// Doubt: the reprompt count of the node the user was reset from is kept.
	if got := harn.repo.state().RepromptCount; got != 1 {
		t.Errorf("RepromptCount = %d, want 1", got)
	}
}

func TestFallbackOnANodeThatIncludesSharedTransitionsIsNotTerminal(t *testing.T) {
	t.Parallel()

	// Built without LoadFlow so the node keeps an include name and an empty transition list.
	flow := flowOf("START", map[string]domain.Node{
		"START": textNode("welcome"),
		"HUB":   {Message: domain.MessageContent{Content: "pick one"}, IncludeTransitions: "shared"},
	})
	harn := newHarness(t, withFlow(flow))
	harn.seed("HUB")

	harn.send("nope")

	harn.wantSent(t, menuFallbackPrefix+"pick one")
	harn.wantNode(t, "HUB")
}

// scriptedFSM answers every message with the generic fallback and serves a fixed set of nodes. The
// real FSM never combines that fallback with a node it cannot find, so it cannot reach that branch.
type scriptedFSM struct {
	start string
	nodes map[string]*domain.Node
}

func (f scriptedFSM) DetermineNext(state *domain.UserState, _ *message.Message) (nodeID, action string) {
	return state.CurrentNode, fallbackResponse
}

func (f scriptedFSM) GetNode(nodeID string) *domain.Node { return f.nodes[nodeID] }

func (f scriptedFSM) GetStartNode() string { return f.start }

func TestFallbackOnAMissingNodeResetsToStart(t *testing.T) {
	t.Parallel()

	fsm := scriptedFSM{start: "START", nodes: map[string]*domain.Node{
		"START": {Message: domain.MessageContent{Content: "welcome"}},
	}}
	harn := newHarness(t, withFSM(fsm))
	harn.seed("GONE")

	harn.send("anything")

	harn.wantSent(t, "welcome")
	harn.wantNode(t, "START")
}

func TestResetToAStartNodeThatDoesNotExist(t *testing.T) {
	t.Parallel()

	fsm := scriptedFSM{start: "MISSING", nodes: map[string]*domain.Node{}}
	harn := newHarness(t, withFSM(fsm))
	harn.seed("GONE")

	harn.send("anything")

	harn.wantSent(t, "Sorry, something went wrong.")
	harn.wantNode(t, "MISSING")
}

func TestActionFailuresEscalate(t *testing.T) {
	t.Parallel()

	setups := []struct {
		name  string
		setup func(*harness) error
	}{
		{"download fails", func(h *harness) error {
			h.wa.downloadErr = errors.New("timeout")

			return nil
		}},
		{"voucher directory cannot be created", func(h *harness) error { return os.WriteFile(h.voucherDir, nil, 0o600) }},
	}

	for _, setup := range setups {
		t.Run(setup.name, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t)
			harn.seed("ENROLLMENT_PROCESS")

			err := setup.setup(harn)
			if err != nil {
				t.Fatal(err)
			}

			harn.bot.HandleEvent(imageEvent(""))

			harn.wantSent(t, voucherFailureText)
			harn.wantTranscript(t,
				storedMessage{"inbound", "ENROLLMENT_PROCESS", ""},
				storedMessage{"outbound", "NEEDS_ASSISTANCE", voucherFailureText},
			)

			state := harn.repo.state()
			if state.CurrentNode != "NEEDS_ASSISTANCE" || !state.RequiresHumanAgent || state.VoucherPath != "" {
				t.Errorf("state = %+v, want escalation without a voucher", state)
			}
		})
	}
}

func TestAnyActionFailureBlamesTheVoucher(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{
		"START":            textNode("start", goTo(exact("go"), "START", "explode")),
		"NEEDS_ASSISTANCE": textNode("help"),
	})
	harn := newHarness(t, withFlow(flow))
	harn.seed("START")

	harn.send("go")

	// Doubt: an unknown action in the flow produces the message about the payment voucher.
	harn.wantSent(t, voucherFailureText)
	harn.wantNode(t, "NEEDS_ASSISTANCE")

	if !harn.repo.state().RequiresHumanAgent {
		t.Error("RequiresHumanAgent not set")
	}
}

func TestSaveFailureForAnExistingUser(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")
	harn.repo.saveErr = errors.New("disk full")

	harn.send("precio")

	harn.wantNode(t, "MAIN_MENU")
	harn.wantSent(t)

	if len(harn.calls.calls) != 1 || harn.calls.calls[0].direction != "inbound" {
		t.Errorf("callbacks = %+v, want only the inbound one", harn.calls.calls)
	}
}

func TestSendFailureForAnExistingUser(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")
	harn.wa.sendErr = errors.New("offline")

	harn.send("precio")

	// Doubt: the reply is stored, called back and the state advanced although it was never delivered.
	harn.wantNode(t, "CONSULTED_PRICE")

	if len(harn.wa.sent) != 1 || len(harn.repo.messages) != 2 || len(harn.calls.calls) != 2 {
		t.Errorf("sent %d, stored %d, callbacks %d; want 1, 2, 2", len(harn.wa.sent), len(harn.repo.messages), len(harn.calls.calls))
	}
}

func TestStateLookupFailureForAnExistingUser(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.seed("MAIN_MENU")
	harn.repo.getStateErr = errors.New("db down")

	harn.send("precio")

	if len(harn.wa.sent) != 0 || harn.repo.saves != 0 || len(harn.calls.calls) != 0 {
		t.Errorf("sent %+v, saves %d, callbacks %+v; want nothing", harn.wa.sent, harn.repo.saves, harn.calls.calls)
	}
}

func TestWithoutCallbackExistingUsersAreStillAnswered(t *testing.T) {
	t.Parallel()

	harn := newHarness(t, withoutCallback())
	harn.seed("MAIN_MENU")

	harn.send("precio")

	harn.wantNode(t, "CONSULTED_PRICE")
}

func TestEnrollmentJourney(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)

	harn.send("hola")
	harn.wantNode(t, "GREETING_INTRO")

	steps := []struct {
		text string
		node string
	}{
		{"1", "INTERESTED_IN_BEGINNER"},
		{"precio", "CONSULTED_PRICE"},
		{"quiero la matricula", "CONFIRM_ENROLLMENT_BEGINNER"},
		{"si", "ENROLLMENT_PROCESS"},
		{"listo", "WAITING_FOR_VOUCHER"},
	}

	for _, step := range steps {
		harn.send(step.text)
		harn.wantNode(t, step.node)
	}

	state := harn.repo.state()
	if state.CourseInterest != "beginner" || !state.ConsultedPrice || state.SelectedCourseID != "CONFIRM_ENROLLMENT_BEGINNER" {
		t.Errorf("lead fields = %+v, want beginner interest, price consulted and the course selected", state)
	}

	harn.bot.HandleEvent(imageEvent("aqui va"))
	harn.wantNode(t, "PAYMENT_CONFIRMED")

	voucher := harn.repo.state().VoucherPath

	saved, err := os.ReadFile(filepath.Clean(voucher))
	if err != nil || string(saved) != "jpeg-bytes" {
		t.Fatalf("voucher %q = %q, %v; want the downloaded bytes", voucher, saved, err)
	}

	// Every further image is saved as well, next to the first one.
	harn.wa.data = []byte("second-jpeg")
	harn.bot.HandleEvent(imageEvent(""))

	harn.wantNode(t, "PAYMENT_CONFIRMED_ACK_EXTRA")

	second := harn.repo.state().VoucherPath
	if harn.wa.downloads != 2 || second == voucher {
		t.Errorf("downloads = %d, VoucherPath = %q; want a second file besides %q", harn.wa.downloads, second, voucher)
	}

	harn.wa.data = []byte("third-jpeg")
	harn.bot.HandleEvent(imageEvent(""))

	harn.wantNode(t, "PAYMENT_CONFIRMED_ACK_EXTRA")

	wantFiles(t, map[string]string{voucher: "jpeg-bytes", second: "second-jpeg", harn.repo.state().VoucherPath: "third-jpeg"})

	harn.send("menu")
	harn.wantNode(t, "MAIN_MENU")
}

// wantFiles checks that each path holds exactly the given content.
func wantFiles(t *testing.T, files map[string]string) {
	t.Helper()

	for path, want := range files {
		got, err := os.ReadFile(filepath.Clean(path))
		if err != nil || string(got) != want {
			t.Errorf("file %q = %q, %v; want %q", path, got, err, want)
		}
	}
}

func TestGreetingCountsOnlyStoredMessages(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)

	// The reply text is rendered before its own exchange is stored: two messages exist when the first
	// menu reply is rendered and four when the second is, so only the second says "welcome back".
	harn.send("hola")
	harn.send("menu")
	harn.send("menu")

	harn.wantSent(t,
		harn.nodeText("GREETING_INTRO", templateData("Ana", welcomeGreeting)),
		harn.nodeText("MAIN_MENU", templateData("Ana", welcomeGreeting)),
		harn.nodeText("MAIN_MENU", templateData("Ana", returningGreeting)),
	)
}
