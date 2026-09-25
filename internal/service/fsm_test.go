//nolint:goconst // Flow node IDs repeat across cases; literals keep each case readable against conversation.json.
package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/internal/service"
)

const (
	fallbackResponse   = "trigger_fallback_response"
	fallbackWrongMedia = "trigger_fallback_wrong_media"
)

type route struct {
	name       string
	node       string
	msg        message.Message
	wantNode   string
	wantAction string
}

func runRoutes(t *testing.T, fsm service.FSM, routes []route) {
	t.Helper()

	for idx := range routes {
		rte := &routes[idx]

		t.Run(rte.name, func(t *testing.T) {
			t.Parallel()

			state := &domain.UserState{UserID: testUserID, CurrentNode: rte.node}

			gotNode, gotAction := fsm.DetermineNext(state, &rte.msg)
			if gotNode != rte.wantNode || gotAction != rte.wantAction {
				t.Errorf("DetermineNext(%q, %+v) = (%q, %q), want (%q, %q)",
					rte.node, rte.msg, gotNode, gotAction, rte.wantNode, rte.wantAction)
			}
		})
	}
}

// Every route below runs against the shipped conversation.json.
func TestDetermineNextRealFlow(t *testing.T) {
	t.Parallel()

	fsm := newTestFSM(loadRealFlow(t))

	t.Run("start menu", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"digit 1", "GREETING_INTRO", textMsg("1"), "INTERESTED_IN_BEGINNER", "create_new_lead"},
			{"digit 2", "GREETING_INTRO", textMsg("2"), "INTERESTED_IN_ADVANCED_CATEGORIES", "create_new_lead"},
			{"keyword with typo", "GREETING_INTRO", textMsg("principiantee"), "INTERESTED_IN_BEGINNER", "create_new_lead"},
			{"input is trimmed and lowercased", "GREETING_INTRO", textMsg("  PRINCIPIANTE \n"), "INTERESTED_IN_BEGINNER", "create_new_lead"},
			// Doubt: keywords match as substrings, so any message containing the digit "1" picks the beginner course.
			{"digit inside a longer sentence", "GREETING_INTRO", textMsg("tengo 21 años"), "INTERESTED_IN_BEGINNER", "create_new_lead"},
			{"short keywords need an exact word", "GREETING_INTRO", textMsg("ok"), "GREETING_INTRO", fallbackResponse},
			{"global keyword", "GREETING_INTRO", textMsg("menu"), "MAIN_MENU", ""},
			{"gibberish", "GREETING_INTRO", textMsg("asdfgh"), "GREETING_INTRO", fallbackResponse},
		})
	})

	t.Run("main menu", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"exact digit", "MAIN_MENU", textMsg("1"), "INTERESTED_IN_BEGINNER", ""},
			{"exact word", "MAIN_MENU", textMsg("Avanzado"), "INTERESTED_IN_ADVANCED_CATEGORIES", ""},
			{"exact does not match a longer text", "MAIN_MENU", textMsg("10"), "MAIN_MENU", fallbackResponse},
			{"online", "MAIN_MENU", textMsg("ONLINE"), "CONSULTED_ONLINE", ""},
			{"presencial", "MAIN_MENU", textMsg("modalidad presencial por favor"), "CONSULTED_PRESENCIAL", ""},
			{"price", "MAIN_MENU", textMsg("precios"), "CONSULTED_PRICE", ""},
			{"price typo", "MAIN_MENU", textMsg("presio"), "CONSULTED_PRICE", ""},
			{"schedule", "MAIN_MENU", textMsg("horarios"), "CONSULTED_SCHEDULE", ""},
			{"enrollment", "MAIN_MENU", textMsg("quiero inscribirme"), "CLARIFY_ENROLLMENT_COURSE", ""},
			{"first listed transition wins", "MAIN_MENU", textMsg("cuanto cuesta la matricula"), "CONSULTED_PRICE", ""},
			// Doubt: "hola" is one edit away from the keyword "hora", so a plain greeting opens the schedule.
			{"greeting is fuzzy-matched to schedule", "MAIN_MENU", textMsg("hola"), "CONSULTED_SCHEDULE", ""},
			{"global help", "MAIN_MENU", textMsg("necesito ayuda"), "NEEDS_ASSISTANCE", ""},
			{"global goodbye", "MAIN_MENU", textMsg("adios"), "CONVERSATION_CLOSED", ""},
			{"gibberish", "MAIN_MENU", textMsg("zzzzzz"), "MAIN_MENU", fallbackResponse},
		})
	})

	t.Run("global name transitions", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"clear name", "MAIN_MENU", textMsg("quita mi nombre"), "NAME_REMOVED_CONFIRMATION", "clear_user_name"},
			{"set name", "MAIN_MENU", textMsg("Me llamo Carla"), "NAME_CHANGE_CONFIRMED", "save_user_name"},
			{"change name has no action", "MAIN_MENU", textMsg("cambiar nombre"), "CHANGE_NAME_PROMPT", ""},
			{"skipped on a node that ignores globals", "COURSE_CORAZON", textMsg("cambiar nombre"), "COURSE_CORAZON", fallbackResponse},
			// Doubt: any_text is a node transition, so it wins over the global help override.
			{"help typed while asked for a name is saved as the name", "CHANGE_NAME_PROMPT", textMsg("ayuda"), "NAME_CHANGE_CONFIRMED", "save_user_name"},
			{"whitespace-only text is not any_text", "CHANGE_NAME_PROMPT", textMsg("   "), "CHANGE_NAME_PROMPT", fallbackResponse},
			{"caption is not any_text", "CHANGE_NAME_PROMPT", mediaMsg("image", "Ana"), "CHANGE_NAME_PROMPT", fallbackResponse},
		})
	})

	t.Run("node action runs when leaving the node", func(t *testing.T) {
		t.Parallel()

		// Doubt: Node.Action is used as the default action of every transition that leaves the
		// node, and is never used for global transitions. Entering a node does not run its action.
		runRoutes(t, fsm, []route{
			{"leaving the beginner node records interest", "INTERESTED_IN_BEGINNER", textMsg("precio"), "CONSULTED_PRICE", "update_lead_interest_beginner"},
			{"common navigation inherits it too", "INTERESTED_IN_BEGINNER", textMsg("menu"), "MAIN_MENU", "update_lead_interest_beginner"},
			{"a global transition skips it", "INTERESTED_IN_BEGINNER", textMsg("adios"), "CONVERSATION_CLOSED", ""},
			{"leaving the advanced node", "INTERESTED_IN_ADVANCED_CATEGORIES", textMsg("catalogo"), "CONSULTED_CATALOG", "update_lead_interest_advanced"},
			{"leaving the price node", "CONSULTED_PRICE", textMsg("horario"), "CONSULTED_SCHEDULE", "update_lead_consulted_price"},
			{"help does not escalate on entry", "MAIN_MENU", textMsg("ayuda"), "NEEDS_ASSISTANCE", ""},
			{"escalation runs when leaving the help node", "NEEDS_ASSISTANCE", textMsg("urgente"), "URGENT_ASSISTANCE", "escalate_to_human_agent"},
			{"asking for help again while in the help node", "NEEDS_ASSISTANCE", textMsg("ayuda"), "NEEDS_ASSISTANCE", "escalate_to_human_agent"},
			{"transition action overrides node action", "CONFIRM_ENROLLMENT_BEGINNER", textMsg("si"), "ENROLLMENT_PROCESS", "set_selected_course"},
		})
	})

	t.Run("advanced courses", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"technique category", "INTERESTED_IN_ADVANCED_CATEGORIES", textMsg("1"), "ADVANCED_MENU_TECHNIQUES", "update_lead_interest_advanced"},
			{"accessories category", "INTERESTED_IN_ADVANCED_CATEGORIES", textMsg("accesorios"), "ADVANCED_MENU_ACCESSORIES", "update_lead_interest_advanced"},
			{"themes category", "INTERESTED_IN_ADVANCED_CATEGORIES", textMsg("3"), "ADVANCED_MENU_THEMES", "update_lead_interest_advanced"},
			{"course from a category menu", "ADVANCED_MENU_TECHNIQUES", textMsg("corazón"), "COURSE_CORAZON", ""},
			{"back from a category menu", "ADVANCED_MENU_TECHNIQUES", textMsg("volver"), "INTERESTED_IN_ADVANCED_CATEGORIES", ""},
			{"course from the catalog", "CONSULTED_CATALOG", textMsg("tiaras"), "COURSE_TIARAS", ""},
			{"enroll in a course", "COURSE_CORAZON", textMsg("quiero inscribirme"), "CONFIRM_ENROLLMENT", "set_selected_course"},
			{"course navigation still works while globals are ignored", "COURSE_CORAZON", textMsg("menu"), "MAIN_MENU", ""},
			{"course help", "COURSE_CORAZON", textMsg("ayuda"), "NEEDS_ASSISTANCE", ""},
			{"course ignores global goodbye", "COURSE_CORAZON", textMsg("adios"), "COURSE_CORAZON", fallbackResponse},
		})
	})

	t.Run("enrollment", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"confirm", "CONFIRM_ENROLLMENT", textMsg("si"), "ENROLLMENT_PROCESS", ""},
			{"cancel", "CONFIRM_ENROLLMENT", textMsg("cancelar"), "ENROLLMENT_CANCELLED", ""},
			// Doubt: "si" and "no" are substrings, so "asi no" (no accent) confirms and "conozco" cancels.
			{"a refusal containing si confirms", "CONFIRM_ENROLLMENT", textMsg("asi no quiero"), "ENROLLMENT_PROCESS", ""},
			{"an answer containing no cancels", "CONFIRM_ENROLLMENT", textMsg("ya conozco el curso"), "ENROLLMENT_CANCELLED", ""},
			{"global goodbye is allowed here", "CONFIRM_ENROLLMENT", textMsg("adios"), "CONVERSATION_CLOSED", ""},
			{"cancelled to beginner", "ENROLLMENT_CANCELLED", textMsg("1"), "INTERESTED_IN_BEGINNER", ""},
			{"cancelled common navigation", "ENROLLMENT_CANCELLED", textMsg("menu"), "MAIN_MENU", ""},
			{"clarify beginner", "CLARIFY_ENROLLMENT_COURSE", textMsg("integral"), "CONFIRM_ENROLLMENT_BEGINNER", ""},
			{"clarify advanced", "CLARIFY_ENROLLMENT_COURSE", textMsg("2"), "PROMPT_ADVANCED_COURSE_FOR_ENROLLMENT", ""},
			{"pick a course to enroll", "PROMPT_ADVANCED_COURSE_FOR_ENROLLMENT", textMsg("flores"), "COURSE_FLORES", ""},
		})
	})

	t.Run("payment", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"paid without voucher", "ENROLLMENT_PROCESS", textMsg("listo"), "WAITING_FOR_VOUCHER", ""},
			{"voucher image", "ENROLLMENT_PROCESS", mediaMsg("image", ""), "PAYMENT_CONFIRMED", "save_payment_voucher"},
			{"media rules beat a caption keyword", "ENROLLMENT_PROCESS", mediaMsg("image", "listo"), "PAYMENT_CONFIRMED", "save_payment_voucher"},
			{"media rules beat a cancelling caption", "ENROLLMENT_PROCESS", mediaMsg("image", "volver"), "PAYMENT_CONFIRMED", "save_payment_voucher"},
			{"voucher image while waiting", "WAITING_FOR_VOUCHER", mediaMsg("image", ""), "PAYMENT_CONFIRMED", "save_payment_voucher"},
			{"own help keyword", "ENROLLMENT_PROCESS", textMsg("ayuda"), "NEEDS_ASSISTANCE", ""},
			{"cancel", "WAITING_FOR_VOUCHER", textMsg("cancelar"), "ENROLLMENT_CANCELLED", ""},
			{"global goodbye ignored", "ENROLLMENT_PROCESS", textMsg("adios"), "ENROLLMENT_PROCESS", fallbackResponse},
			{"global help override is allowed", "ENROLLMENT_PROCESS", textMsg("quiero hablar con una asesora"), "NEEDS_ASSISTANCE", ""},
			{"wrong media type", "ENROLLMENT_PROCESS", mediaMsg("video", ""), "ENROLLMENT_PROCESS", fallbackWrongMedia},
			{"wrong media type with a caption that matches a node keyword", "ENROLLMENT_PROCESS", mediaMsg("video", "ayuda"), "NEEDS_ASSISTANCE", ""},
			{"wrong media type with a global help caption", "WAITING_FOR_VOUCHER", mediaMsg("document", "necesito una asesora"), "NEEDS_ASSISTANCE", ""},
			// Doubt: the voucher node has no action, so a second image is acknowledged but never saved.
			{"second image", "PAYMENT_CONFIRMED", mediaMsg("image", ""), "PAYMENT_CONFIRMED_ACK_EXTRA", ""},
			{"text after payment", "PAYMENT_CONFIRMED", textMsg("gracias"), "PAYMENT_CONFIRMED", fallbackResponse},
			{"menu after payment", "PAYMENT_CONFIRMED_ACK_EXTRA", textMsg("menu"), "MAIN_MENU", ""},
		})
	})

	t.Run("names", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"any text is the new name", "CHANGE_NAME_PROMPT", textMsg("Lucía"), "NAME_CHANGE_CONFIRMED", "save_user_name"},
			{"name confirmation reuses the main menu", "NAME_CHANGE_CONFIRMED", textMsg("precio"), "CONSULTED_PRICE", ""},
			{"name removed confirmation", "NAME_REMOVED_CONFIRMATION", textMsg("menu"), "MAIN_MENU", ""},
		})
	})

	t.Run("closing", func(t *testing.T) {
		t.Parallel()

		runRoutes(t, fsm, []route{
			{"any text reopens the menu", "CONVERSATION_CLOSED", textMsg("hola de nuevo"), "MAIN_MENU", ""},
			{"a caption does not reopen it", "CONVERSATION_CLOSED", mediaMsg("image", "hola"), "CONVERSATION_CLOSED", fallbackResponse},
			{"urgent node still honors globals", "URGENT_ASSISTANCE", textMsg("menu"), "MAIN_MENU", ""},
			{"urgent node fallback", "URGENT_ASSISTANCE", textMsg("gracias"), "URGENT_ASSISTANCE", fallbackResponse},
			{"unsupported media without a transition", "MAIN_MENU", mediaMsg("audio", ""), "MAIN_MENU", fallbackResponse},
			{"image without a transition", "MAIN_MENU", mediaMsg("image", ""), "MAIN_MENU", fallbackResponse},
			{"global keyword in a caption", "MAIN_MENU", mediaMsg("image", "ayuda"), "NEEDS_ASSISTANCE", ""},
			{"unknown node resets to start", "GHOST", textMsg("precio"), "GREETING_INTRO", ""},
		})
	})
}

func TestDetermineNextInlineFlows(t *testing.T) {
	t.Parallel()

	t.Run("text messages", func(t *testing.T) {
		t.Parallel()

		nodes := map[string]domain.Node{
			"A": {
				Message: domain.MessageContent{Content: "a"},
				Action:  "node_action",
				Transitions: []domain.Transition{
					goTo(keyword("first"), "B", "own_action"),
					goTo(keyword("first", "second"), "C", ""),
					goTo(mediaType("image"), "D", ""),
					goTo(anyMedia(), "D", ""),
				},
			},
			"B": textNode("b"),
			"C": textNode("c"),
			"D": textNode("d"),
		}

		runRoutes(t, newTestFSM(flowOf("A", nodes)), []route{
			{"first match wins and uses its action", "A", textMsg("first"), "B", "own_action"},
			{"empty action falls back to the node action", "A", textMsg("second"), "C", "node_action"},
			{"media conditions never match text", "A", textMsg("third"), "A", fallbackResponse},
		})
	})

	t.Run("media messages", func(t *testing.T) {
		t.Parallel()

		nodes := map[string]domain.Node{
			"A": {
				Message: domain.MessageContent{Content: "a"},
				Action:  "node_action",
				Transitions: []domain.Transition{
					goTo(keyword("caption"), "TEXT", ""),
					goTo(keyword("explicit"), "TEXT_EXPLICIT", "caption_action"),
					goTo(mediaType("image"), "IMAGE", ""),
					goTo(mediaType("video"), "VIDEO", "video_action"),
				},
			},
			"IMAGE":         textNode("i"),
			"VIDEO":         textNode("v"),
			"TEXT":          textNode("t"),
			"TEXT_EXPLICIT": textNode("e"),
			"ANY": {
				Message:     domain.MessageContent{Content: "any"},
				Action:      "any_action",
				Transitions: []domain.Transition{goTo(anyMedia(), "IMAGE", "")},
			},
		}
		fsm := newTestFSM(flowOf("A", nodes))

		runRoutes(t, fsm, []route{
			{"media transition without an action uses the node action", "A", mediaMsg("image", ""), "IMAGE", "node_action"},
			{"media transition with its own action", "A", mediaMsg("video", ""), "VIDEO", "video_action"},
			{"media transitions are tried before earlier caption transitions", "A", mediaMsg("image", "caption"), "IMAGE", "node_action"},
			{"caption transition without an action uses the node action", "A", mediaMsg("audio", "a caption"), "TEXT", "node_action"},
			{"caption transition with its own action", "A", mediaMsg("audio", "explicit"), "TEXT_EXPLICIT", "caption_action"},
			{"the media condition matches any media", "ANY", mediaMsg("sticker", ""), "IMAGE", "any_action"},
			{"no caption and no matching media type", "A", mediaMsg("audio", ""), "A", fallbackWrongMedia},
		})
	})

	t.Run("global transitions", func(t *testing.T) {
		t.Parallel()

		nodes := map[string]domain.Node{
			"OPEN":             {Message: domain.MessageContent{Content: "o"}, Action: "node_action", Transitions: []domain.Transition{goTo(keyword("local"), "LOCAL", "")}},
			"CLOSED":           {Message: domain.MessageContent{Content: "c"}, IgnoreGlobalTransitions: true, Transitions: []domain.Transition{goTo(keyword("local"), "LOCAL", "")}},
			"LOCAL":            textNode("l"),
			"GLOBAL":           textNode("g"),
			"NEEDS_ASSISTANCE": textNode("help"),
		}
		globals := []domain.Transition{
			goTo(keyword("both"), "GLOBAL", "first_global"),
			goTo(keyword("both"), "LOCAL", "second_global"),
			goTo(keyword("help"), "NEEDS_ASSISTANCE", "help_action"),
			goTo(keyword("local"), "GLOBAL", ""),
		}
		fsm := newTestFSM(flowOf("OPEN", nodes, globals...))

		runRoutes(t, fsm, []route{
			{"node transitions beat globals", "OPEN", textMsg("local"), "LOCAL", "node_action"},
			{"first global wins and the node action is not used", "OPEN", textMsg("both"), "GLOBAL", "first_global"},
			{"help global on an open node", "OPEN", textMsg("help"), "NEEDS_ASSISTANCE", "help_action"},
			{"closed node skips non-help globals", "CLOSED", textMsg("both"), "CLOSED", fallbackResponse},
			{"closed node still allows the help global", "CLOSED", textMsg("help"), "NEEDS_ASSISTANCE", "help_action"},
			{"globals also apply to captions", "OPEN", mediaMsg("image", "both"), "GLOBAL", "first_global"},
			{"closed node ignores globals in captions", "CLOSED", mediaMsg("image", "both"), "CLOSED", fallbackResponse},
			{"nothing matches", "OPEN", textMsg("zzzz"), "OPEN", fallbackResponse},
		})
	})

	t.Run("wrong media type", func(t *testing.T) {
		t.Parallel()

		nodes := map[string]domain.Node{
			"WANTS_VIDEO": textNode("v", goTo(mediaType("video"), "DONE", "")),
			"NO_MEDIA":    textNode("n", goTo(keyword("x"), "DONE", "")),
			"DONE":        textNode("d"),
		}
		globals := []domain.Transition{goTo(keyword("menu"), "DONE", "")}
		fsm := newTestFSM(flowOf("WANTS_VIDEO", nodes, globals...))

		runRoutes(t, fsm, []route{
			{"other media type", "WANTS_VIDEO", mediaMsg("image", ""), "WANTS_VIDEO", fallbackWrongMedia},
			{"expected media type", "WANTS_VIDEO", mediaMsg("video", ""), "DONE", ""},
			{"a global match beats the wrong media fallback", "WANTS_VIDEO", mediaMsg("image", "menu"), "DONE", ""},
			{"text is never wrong media", "WANTS_VIDEO", textMsg("hello"), "WANTS_VIDEO", fallbackResponse},
			{"a node without media rules gives the generic fallback", "NO_MEDIA", mediaMsg("image", ""), "NO_MEDIA", fallbackResponse},
		})
	})

	t.Run("start of the flow", func(t *testing.T) {
		t.Parallel()

		nodes := map[string]domain.Node{"START": textNode("s"), "OTHER": textNode("o")}

		runRoutes(t, newTestFSM(flowOf("START", nodes)), []route{
			{"unknown current node resets to the start node without an action", "GONE", textMsg("anything"), "START", ""},
			{"a known node with no transitions gives the generic fallback", "OTHER", textMsg("anything"), "OTHER", fallbackResponse},
		})
	})
}

type conditionCase struct {
	name string
	cond domain.Condition
	msg  message.Message
	want bool
}

// Conditions are unexported, so each case runs through a one-transition flow.
func runConditions(t *testing.T, cases []conditionCase) {
	t.Helper()

	for idx := range cases {
		testCase := &cases[idx]

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			nodes := map[string]domain.Node{"SRC": textNode("s", goTo(testCase.cond, "HIT", "")), "HIT": textNode("h")}
			state := &domain.UserState{UserID: testUserID, CurrentNode: "SRC"}

			gotNode, _ := newTestFSM(flowOf("SRC", nodes)).DetermineNext(state, &testCase.msg)
			if got := gotNode == "HIT"; got != testCase.want {
				t.Errorf("condition %+v on %+v matched = %v, want %v", testCase.cond, testCase.msg, got, testCase.want)
			}
		})
	}
}

func TestConditionExact(t *testing.T) {
	t.Parallel()

	runConditions(t, []conditionCase{
		{"one of several values", exact("1", "uno"), textMsg("uno"), true},
		{"case-insensitive on both sides", exact("Sí"), textMsg("SÍ"), true},
		{"surrounding whitespace is trimmed", exact("1"), textMsg("  1\n"), true},
		{"not a substring", exact("1"), textMsg("10"), false},
		{"not a prefix", exact("uno"), textMsg("uno mas"), false},
		{"accents are significant", exact("basico"), textMsg("básico"), false},
		{"no values", exact(), textMsg("1"), false},
		{"empty text matches an empty value", exact(""), textMsg("   "), true},
		{"a caption is compared like text", exact("listo"), mediaMsg("image", "Listo"), true},
	})
}

func TestConditionKeywordSubstring(t *testing.T) {
	t.Parallel()

	runConditions(t, []conditionCase{
		{"substring of a sentence", keyword("help"), textMsg("i need help please"), true},
		{"keyword case is ignored", keyword("HELP"), textMsg("help"), true},
		{"second keyword", keyword("nothing", "help"), textMsg("help"), true},
		{"multi-word keyword", keyword("hasta luego"), textMsg("bueno, hasta luego amigo"), true},
		{"no keyword present", keyword("help"), textMsg("all fine"), false},
		{"no keywords", keyword(), textMsg("help"), false},
		{"empty text", keyword("x"), textMsg(""), false},
		// Doubt: a keyword found inside another word matches, e.g. "no" in "conozco" or "si" in "presion".
		{"inside another word", keyword("si"), textMsg("presion"), true},
		// Doubt: an empty keyword is a substring of everything, including an empty message.
		{"empty keyword matches anything", keyword(""), textMsg("whatever"), true},
		{"empty keyword matches empty text", keyword(""), textMsg(""), true},
		{"caption is searched", keyword("ayuda"), mediaMsg("image", "necesito ayuda"), true},
	})
}

func TestConditionKeywordFuzzy(t *testing.T) {
	t.Parallel()

	runConditions(t, []conditionCase{
		// Words of 5+ characters against keywords of 5+ characters allow two edits.
		{"one edit, long words", keyword("precio"), textMsg("presio"), true},
		{"two edits, long words", keyword("precio"), textMsg("presiu"), true},
		{"three edits, long words", keyword("precio"), textMsg("presuu"), false},
		{"only the misspelled word is compared", keyword("precio"), textMsg("cual es el presio"), true},
		{"accent difference", keyword("atrás"), textMsg("atras"), true},
		{"equal length five-letter words", keyword("costo"), textMsg("cxxto"), true},
		{"three edits, five-letter words", keyword("costo"), textMsg("cxxxo"), false},

		// A word or keyword of 3-4 characters allows one edit.
		{"one edit, short keyword", keyword("hora"), textMsg("hola"), true},
		{"two edits, short keyword", keyword("hora"), textMsg("hxxa"), false},
		{"short word against a long keyword", keyword("costo"), textMsg("cost"), true},
		{"two edits against a long keyword", keyword("costo"), textMsg("cos"), false},
		{"short keyword, longer word", keyword("gato"), textMsg("gatxo"), true},
		{"short keyword, longer word, two edits", keyword("gato"), textMsg("gatxx"), false},

		// A word or keyword of 1-2 characters must match exactly.
		{"two-letter word never fuzzy-matches", keyword("abc"), textMsg("ab"), false},
		{"two-letter keyword never fuzzy-matches", keyword("ok"), textMsg("oik"), false},
		{"one-letter keyword", keyword("1"), textMsg("l"), false},

		{"multi-word keywords are only matched as substrings", keyword("hasta luego"), textMsg("hasta lugo"), false},
		// Doubt: the length thresholds count bytes, so the two-letter word "ñó" (4 bytes) gets one edit.
		{"thresholds count bytes, not letters", keyword("ñóx"), textMsg("ñó"), true},
	})
}

func TestConditionRegex(t *testing.T) {
	t.Parallel()

	runConditions(t, []conditionCase{
		{"match", regex(`^\d{3}$`), textMsg("123"), true},
		{"no match", regex(`^\d{3}$`), textMsg("12a"), false},
		{"unanchored", regex(`ord(en|er)`), textMsg("mi orden llego"), true},
		{"input is lowercased first", regex(`^hola$`), textMsg("HOLA"), true},
		// Doubt: the input is lowercased but the pattern is not, so a pattern with capitals never matches.
		{"pattern with capitals never matches", regex(`^Hola$`), textMsg("Hola"), false},
		{"empty pattern", regex(""), textMsg("anything"), false},
		{"invalid pattern", regex("("), textMsg("("), false},
		{"regex ignores the value list", domain.Condition{Type: "regex", Value: []string{"hola"}, Regex: "chau"}, textMsg("hola"), false},
		{"caption is matched", regex(`^\d+$`), mediaMsg("image", "42"), true},
	})
}

func TestConditionAnyTextMediaAndMediaType(t *testing.T) {
	t.Parallel()

	runConditions(t, []conditionCase{
		{"any_text with text", anyText(), textMsg("hola"), true},
		{"any_text with empty text", anyText(), textMsg(""), false},
		{"any_text with whitespace", anyText(), textMsg(" \t"), false},
		{"any_text with a captioned image", anyText(), mediaMsg("image", "caption"), false},
		{"media with an image", anyMedia(), mediaMsg("image", ""), true},
		{"media with a sticker", anyMedia(), mediaMsg("sticker", ""), true},
		{"media with text", anyMedia(), textMsg("hola"), false},
		{"media_type match", mediaType("image"), mediaMsg("image", ""), true},
		{"media_type among several", mediaType("video", "image"), mediaMsg("image", ""), true},
		{"media_type mismatch", mediaType("image"), mediaMsg("video", ""), false},
		{"media_type with text", mediaType("image"), textMsg("hola"), false},
		{"media_type without values", mediaType(), mediaMsg("image", ""), false},
		{"unknown condition type", domain.Condition{Type: "sentiment", Value: []string{"hola"}}, textMsg("hola"), false},
		{"empty condition type", domain.Condition{Value: []string{"hola"}}, textMsg("hola"), false},
	})
}

func TestRegexIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()

	nodes := map[string]domain.Node{
		"SRC": textNode("s", goTo(regex(`^\d+$`), "HIT", "")),
		"HIT": textNode("h"),
	}
	fsm := newTestFSM(flowOf("SRC", nodes))

	const workers = 32

	var group sync.WaitGroup

	results := make([]string, workers)

	for worker := range workers {
		group.Go(func() {
			state := &domain.UserState{UserID: testUserID, CurrentNode: "SRC"}
			msg := textMsg("42")
			results[worker], _ = fsm.DetermineNext(state, &msg)
		})
	}

	group.Wait()

	for worker, got := range results {
		if got != "HIT" {
			t.Errorf("worker %d ended in %q, want HIT", worker, got)
		}
	}
}

func TestGetNodeAndStartNode(t *testing.T) {
	t.Parallel()

	flow := loadRealFlow(t)
	fsm := newTestFSM(flow)

	if got := fsm.GetStartNode(); got != "GREETING_INTRO" {
		t.Errorf("GetStartNode() = %q, want GREETING_INTRO", got)
	}

	node := fsm.GetNode("MAIN_MENU")
	if node == nil || !strings.Contains(node.Message.Content, "{{greeting}}") {
		t.Fatalf("GetNode(MAIN_MENU) = %+v, want the menu node", node)
	}

	// The node is a copy: rewriting it must not change the flow.
	node.Message.Content = "changed"

	if got := fsm.GetNode("MAIN_MENU").Message.Content; got == "changed" {
		t.Error("GetNode returned a pointer into the flow, not a copy")
	}

	if got := fsm.GetNode("NOPE"); got != nil {
		t.Errorf("GetNode(NOPE) = %+v, want nil", got)
	}
}

func writeFlowFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "flow.json")

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadFlowShippedFlow(t *testing.T) {
	t.Parallel()

	flow := loadRealFlow(t)

	if flow.StartNode != "GREETING_INTRO" {
		t.Errorf("start node = %q", flow.StartNode)
	}

	// INTERESTED_IN_BEGINNER lists 3 own transitions after the 2 shared ones.
	transitions := flow.Nodes["INTERESTED_IN_BEGINNER"].Transitions
	if len(transitions) != 5 || transitions[0].Target != "MAIN_MENU" || transitions[4].Target != "CONSULTED_PRESENCIAL" {
		t.Errorf("merged transitions = %+v, want the shared group first and own transitions last", transitions)
	}
}

func TestLoadFlowFailures(t *testing.T) {
	t.Parallel()

	node := `{"message": {"type": "text", "content": "hi"}}`
	cases := []struct {
		name    string
		content string
		wantErr string
	}{
		{"invalid json", `{`, "failed to parse flow file"},
		{"empty start node", `{"nodes": {"A": ` + node + `}}`, "start_node cannot be empty"},
		{"unknown start node", `{"start_node": "B", "nodes": {"A": ` + node + `}}`, "start_node not found"},
		{
			"unknown global target",
			`{"start_node": "A", "nodes": {"A": ` + node + `}, "global_transitions": [{"condition": {"type": "any_text"}, "target": "Z"}]}`,
			`invalid transition from global_transitions to non-existent node "Z"`,
		},
		{
			"unknown node target",
			`{"start_node": "A", "nodes": {"A": {"transitions": [{"condition": {"type": "any_text"}, "target": "Z"}]}}}`,
			`invalid transition from node "A" to non-existent node "Z"`,
		},
		{
			"unknown transition group",
			`{"start_node": "A", "nodes": {"A": {"include_transitions": "nope"}}, "transition_groups": {"g": []}}`,
			`includes non-existent transition group "nope"`,
		},
		{
			"a group with an unknown target",
			`{"start_node": "A", "nodes": {"A": {"include_transitions": "g"}}, "transition_groups": {"g": [{"condition": {"type": "any_text"}, "target": "Z"}]}}`,
			`invalid transition from node "A" to non-existent node "Z"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.LoadFlow(writeFlowFile(t, testCase.content))
			if err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Errorf("LoadFlow error = %v, want it to contain %q", err, testCase.wantErr)
			}
		})
	}
}

func TestLoadFlowMissingFile(t *testing.T) {
	t.Parallel()

	_, err := service.LoadFlow(filepath.Join(t.TempDir(), "absent.json"))
	if err == nil || !strings.Contains(err.Error(), "failed to read flow file") {
		t.Errorf("LoadFlow error = %v, want a read failure", err)
	}
}

func TestLoadFlowIncludeWithoutAnyTransitionGroups(t *testing.T) {
	t.Parallel()

	// Doubt: when the flow defines no transition_groups at all, include_transitions is not
	// checked, so a typo in a group name loads fine and the node silently gets no shared transitions.
	flow, err := service.LoadFlow(writeFlowFile(t, `{"start_node": "A", "nodes": {"A": {"include_transitions": "typo"}}}`))
	if err != nil {
		t.Fatalf("LoadFlow: %v", err)
	}

	if got := flow.Nodes["A"].Transitions; len(got) != 0 {
		t.Errorf("transitions = %+v, want none", got)
	}
}
