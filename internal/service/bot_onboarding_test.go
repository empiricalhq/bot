//nolint:goconst // Flow node IDs repeat across cases; literals keep each case readable against conversation.json.
package service_test

import (
	"errors"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/config"
	"whatsbot/internal/domain"
)

func TestHandleEventFiltering(t *testing.T) {
	t.Parallel()

	devConfig := func(allowed ...string) *config.Config {
		cfg := &config.Config{Environment: "dev", DevAllowedUsers: map[string]bool{}}
		for _, user := range allowed {
			cfg.DevAllowedUsers[user] = true
		}

		return cfg
	}
	fromMe := func(deviceSent bool) *events.Message {
		evt := textEvent("hola")
		evt.Info.IsFromMe = true

		if deviceSent {
			evt.Info.DeviceSentMeta = &types.DeviceSentMeta{}
		}

		return evt
	}
	inChat := func(server string) *events.Message {
		evt := textEvent("hola")
		evt.Info.Chat = types.NewJID(testPhone, server)

		return evt
	}

	prod := &config.Config{Environment: "prod"}
	prodWithAllowList := &config.Config{Environment: "prod", DevAllowedUsers: map[string]bool{"someone-else": true}}

	cases := []struct {
		name string
		cfg  *config.Config
		evt  any
		want bool // whether the bot answers
	}{
		{"direct text message", prod, textEvent("hola"), true},
		{"extended text message", prod, newEvent(extendedText("hola")), true},
		{"hidden-user server", prod, inChat(types.HiddenUserServer), true},
		{"group chat", prod, inChat(types.GroupServer), false},
		{"broadcast list", prod, inChat(types.BroadcastServer), false},
		{"event that is not a message", prod, &events.Connected{}, false},
		{"nil event", prod, nil, false},
		{"message without text or media", prod, emptyEvent(), false},
		{"blank text", prod, textEvent("   "), false},
		{"own message in prod", prod, fromMe(false), false},
		{"own message from another device in prod", prod, fromMe(true), false},
		{"own message in dev without device metadata", devConfig(testUserID), fromMe(false), false},
		// Doubt: SenderID comes from the chat, so in dev the bot answers the other party of a chat you typed in.
		{"own message from another device in dev", devConfig(testUserID), fromMe(true), true},
		{"dev with no allow list ignores everyone", devConfig(), textEvent("hola"), false},
		{"dev ignores users outside the allow list", devConfig("51000@s.whatsapp.net"), textEvent("hola"), false},
		{"dev answers allowed users", devConfig(testUserID), textEvent("hola"), true},
		{"the allow list only applies in dev", prodWithAllowList, textEvent("hola"), true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t, withConfig(testCase.cfg))

			harn.bot.HandleEvent(testCase.evt)

			if got := len(harn.wa.sent) == 1; got != testCase.want {
				t.Errorf("answered = %v, want %v (sent %+v)", got, testCase.want, harn.wa.sent)
			}

			if !testCase.want && (harn.repo.saves != 0 || len(harn.calls.calls) != 0) {
				t.Errorf("ignored event still saved %d times and called back %+v", harn.repo.saves, harn.calls.calls)
			}
		})
	}
}

func TestHandleEventAnswersOnTheChatJID(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	evt := textEvent("hola")
	evt.Info.Chat = types.NewJID(testPhone, types.HiddenUserServer)

	harn.bot.HandleEvent(evt)

	if len(harn.wa.sent) != 1 || harn.wa.sent[0].to != testPhone+"@lid" {
		t.Errorf("sent = %+v, want one message to %s@lid", harn.wa.sent, testPhone)
	}
}

func TestNewUserIsGreeted(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)

	harn.send("hola")

	want := harn.nodeText("GREETING_INTRO", templateData("Ana", welcomeGreeting))
	harn.wantSent(t, want)

	state := harn.repo.state()
	if state.CurrentNode != "GREETING_INTRO" || state.UserName != testName || state.RepromptCount != 0 {
		t.Errorf("state = %+v, want GREETING_INTRO with the WhatsApp push name", state)
	}

	if time.Since(state.LastUpdated) > time.Minute {
		t.Errorf("LastUpdated = %v, want now", state.LastUpdated)
	}

	harn.wantTranscript(t,
		storedMessage{"inbound", "GREETING_INTRO", "hola"},
		storedMessage{"outbound", "GREETING_INTRO", want},
	)

	wantCalls := []callbackCall{{"outbound", testUserID, "Bot", want}}
	if len(harn.calls.calls) != 1 || harn.calls.calls[0] != wantCalls[0] {
		t.Errorf("callbacks = %+v, want %+v", harn.calls.calls, wantCalls)
	}
}

func TestNewUserFirstMessageThatAnswersTheMenuIsActedOn(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text string
		node string
	}{
		{"1", "INTERESTED_IN_BEGINNER"},
		{"quiero empezar desde cero", "INTERESTED_IN_BEGINNER"},
		{"2", "INTERESTED_IN_ADVANCED_CATEGORIES"},
		{"necesito ayuda", "NEEDS_ASSISTANCE"},
	}

	for _, testCase := range cases {
		t.Run(testCase.text, func(t *testing.T) {
			t.Parallel()

			harn := newHarness(t)

			harn.send(testCase.text)

			greeting := harn.nodeText("GREETING_INTRO", templateData("Ana", welcomeGreeting))
			answer := harn.nodeText(testCase.node, templateData("Ana", welcomeGreeting))

			harn.wantSent(t, greeting, answer)
			harn.wantNode(t, testCase.node)
			harn.wantTranscript(t,
				storedMessage{"inbound", "GREETING_INTRO", testCase.text},
				storedMessage{"outbound", "GREETING_INTRO", greeting},
				storedMessage{"outbound", testCase.node, answer},
			)
		})
	}
}

func TestNewUserFirstMessageRunsTheActionsOfTheNodeItReaches(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)

	harn.send("1")

	if got := harn.repo.state().CourseInterest; got != "beginner" {
		t.Errorf("CourseInterest = %q, want beginner", got)
	}
}

func TestNewUserFirstMessageThatDoesNotAnswerTheMenuIsOnlyStored(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)

	harn.send("buenas tardes, tengo 21 años")

	greeting := harn.nodeText("GREETING_INTRO", templateData("Ana", welcomeGreeting))
	harn.wantSent(t, greeting)
	harn.wantNode(t, "GREETING_INTRO")
	harn.wantTranscript(t,
		storedMessage{"inbound", "GREETING_INTRO", "buenas tardes, tengo 21 años"},
		storedMessage{"outbound", "GREETING_INTRO", greeting},
	)

	if got := harn.repo.state().RepromptCount; got != 0 {
		t.Errorf("RepromptCount = %d, want the unanswered first message not to count as a fallback", got)
	}
}

func TestNewUserEntersTheStartNodeAndRunsItsAction(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{
		"START": {Message: domain.MessageContent{Content: "welcome"}, Action: "escalate_to_human_agent"},
	})
	harn := newHarness(t, withFlow(flow))

	harn.send("hola")

	if !harn.repo.state().RequiresHumanAgent {
		t.Error("the start node action did not run when the new user entered it")
	}
}

func TestNewUserWithoutUsablePushNameIsCalledAmigx(t *testing.T) {
	t.Parallel()

	for _, pushName := range []string{"", "xyz", "12"} {
		harn := newHarness(t)
		evt := textEvent("hola")
		evt.Info.PushName = pushName

		harn.bot.HandleEvent(evt)

		harn.wantSent(t, harn.nodeText("GREETING_INTRO", templateData("amigx", welcomeGreeting)))

		if got := harn.repo.state().UserName; got != pushName {
			t.Errorf("stored name = %q, want the raw push name %q", got, pushName)
		}
	}
}

func TestNewUserOnAStartNodeWithoutMessage(t *testing.T) {
	t.Parallel()

	flow := flowOf("START", map[string]domain.Node{"START": {}})
	harn := newHarness(t, withFlow(flow))

	harn.send("hola")

	harn.wantSent(t)
	harn.wantNode(t, "START")

	// Doubt: an empty outbound message is stored even though nothing is sent.
	harn.wantTranscript(t, storedMessage{"inbound", "START", "hola"}, storedMessage{"outbound", "START", ""})

	if len(harn.calls.calls) != 0 {
		t.Errorf("callbacks = %+v, want none", harn.calls.calls)
	}
}

func TestNewUserWithoutCallback(t *testing.T) {
	t.Parallel()

	harn := newHarness(t, withoutCallback())

	harn.send("hola")

	if len(harn.wa.sent) != 1 {
		t.Errorf("sent = %+v, want the greeting", harn.wa.sent)
	}
}

func TestOnboardingFailures(t *testing.T) {
	t.Parallel()

	t.Run("state lookup fails", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.repo.getStateErr = errors.New("db down")

		harn.send("hola")

		if len(harn.wa.sent) != 0 || harn.repo.saves != 0 {
			t.Errorf("sent %+v and saved %d times, want neither", harn.wa.sent, harn.repo.saves)
		}
	})

	t.Run("saving the new user fails", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.repo.saveErr = errors.New("disk full")

		harn.send("hola")

		if len(harn.wa.sent) != 0 || len(harn.calls.calls) != 0 {
			t.Errorf("sent %+v, callbacks %+v; want no greeting when the save fails", harn.wa.sent, harn.calls.calls)
		}

		// The next message starts onboarding again.
		harn.repo.saveErr = nil

		harn.send("hola")

		harn.wantNode(t, "GREETING_INTRO")

		if len(harn.wa.sent) != 1 {
			t.Errorf("sent %d messages, want the greeting once", len(harn.wa.sent))
		}
	})

	t.Run("sending the greeting fails", func(t *testing.T) {
		t.Parallel()

		harn := newHarness(t)
		harn.wa.sendErr = errors.New("offline")

		harn.send("hola")

		// Doubt: the user counts as onboarded (state saved, callback fired) although the
		// greeting never arrived, and the next message is already treated as a menu answer.
		harn.wantNode(t, "GREETING_INTRO")

		if len(harn.wa.sent) != 1 || len(harn.calls.calls) != 1 {
			t.Errorf("sent %d, callbacks %d; want the failed attempt and its callback", len(harn.wa.sent), len(harn.calls.calls))
		}

		harn.wa.sendErr = nil

		harn.send("1")

		harn.wantNode(t, "INTERESTED_IN_BEGINNER")
	})
}

func seedStaleUser(harn *harness) {
	harn.repo.seed(domain.UserState{
		UserID:           testUserID,
		CurrentNode:      "INTERESTED_IN_BEGINNER",
		UserName:         "Carla",
		SelectedCourseID: "COURSE_MAMA",
		CourseInterest:   "beginner",
		RepromptCount:    2,
		LastUpdated:      time.Now().Add(-25 * time.Hour),
	})
}

// A stale conversation is moved back to the start node, but unlike a new user the person is not
// greeted: getOrCreateUserState only greets when CurrentNode is empty, and the reset just set it.
func TestStaleConversationIsMovedToTheStartNodeWithoutAGreeting(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	seedStaleUser(harn)

	harn.send("precio")

	// Doubt: the first message after a pause is interpreted against the start menu the user has not
	// just seen, so a natural question gets "No entendí tu respuesta" followed by the greeting.
	want := harn.render(menuFallbackPrefix+harn.flow.Nodes["GREETING_INTRO"].Message.Content, templateData("Carla", welcomeGreeting))
	harn.wantSent(t, want)
	harn.wantTranscript(t,
		storedMessage{"inbound", "GREETING_INTRO", "precio"},
		storedMessage{"outbound", "GREETING_INTRO", want},
	)

	// The name and course fields are kept. The reset count of 0 was incremented by the fallback.
	state := harn.repo.state()
	if state.CurrentNode != "GREETING_INTRO" || state.RepromptCount != 1 {
		t.Errorf("state = %+v, want the start node with reprompt count 1", state)
	}

	if state.UserName != "Carla" || state.SelectedCourseID != "COURSE_MAMA" || state.CourseInterest != "beginner" {
		t.Errorf("state = %+v, want the name and course fields kept", state)
	}
}

func TestStaleConversationAnswersTheStartMenuDirectly(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	seedStaleUser(harn)

	harn.send("1")

	harn.wantNode(t, "INTERESTED_IN_BEGINNER")

	if state := harn.repo.state(); state.RepromptCount != 0 || state.UserName != "Carla" {
		t.Errorf("state = %+v, want reprompt count 0 and the name kept", state)
	}
}

func TestConversationWithinADayIsNotStale(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.repo.seed(domain.UserState{
		UserID:      testUserID,
		CurrentNode: "MAIN_MENU",
		UserName:    "Carla",
		LastUpdated: time.Now().Add(-23 * time.Hour),
	})

	harn.send("precio")

	harn.wantNode(t, "CONSULTED_PRICE")

	if got := harn.repo.state().UserName; got != "Carla" {
		t.Errorf("UserName = %q, want it kept", got)
	}
}

func TestStateWithoutANodeIsOnboardedEvenIfOld(t *testing.T) {
	t.Parallel()

	harn := newHarness(t)
	harn.repo.seed(domain.UserState{UserID: testUserID, UserName: "Old", LastUpdated: time.Now().Add(-72 * time.Hour)})

	harn.send("hola")

	harn.wantNode(t, "GREETING_INTRO")

	if got := harn.repo.state().UserName; got != testName {
		t.Errorf("UserName = %q, want the push name", got)
	}
}
