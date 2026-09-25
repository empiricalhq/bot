//nolint:goconst // Flow node IDs and names repeat across table rows; literals keep each row readable.
package service_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/internal/service"
)

type actionEnv struct {
	handler service.ActionHandler
	wa      *fakeWA
}

func newActionEnv(voucherDir string) actionEnv {
	waClient := &fakeWA{data: []byte("jpeg-bytes")}

	return actionEnv{handler: service.NewActionHandler(discardLogger(), waClient, voucherDir), wa: waClient}
}

func TestExecuteSimpleActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		action string
		before domain.UserState
		origin string
		want   domain.UserState
	}{
		{"empty action does nothing", "", domain.UserState{UserName: "Ana"}, "N", domain.UserState{UserName: "Ana"}},
		{"create_new_lead changes no state", "create_new_lead", domain.UserState{UserName: "Ana"}, "N", domain.UserState{UserName: "Ana"}},
		{"beginner interest", "update_lead_interest_beginner", domain.UserState{}, "N", domain.UserState{CourseInterest: "beginner"}},
		{"advanced interest replaces beginner", "update_lead_interest_advanced", domain.UserState{CourseInterest: "beginner"}, "N", domain.UserState{CourseInterest: "advanced"}},
		{"consulted price", "update_lead_consulted_price", domain.UserState{}, "N", domain.UserState{ConsultedPrice: true}},
		{"escalate", "escalate_to_human_agent", domain.UserState{}, "N", domain.UserState{RequiresHumanAgent: true}},
		{"clear name", "clear_user_name", domain.UserState{UserName: "Ana"}, "N", domain.UserState{}},
		{"selected course is the originating node", "set_selected_course", domain.UserState{}, "COURSE_MAMA", domain.UserState{SelectedCourseID: "COURSE_MAMA"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			env := newActionEnv(t.TempDir())
			state := testCase.before
			state.UserID = testUserID
			testCase.want.UserID = testUserID

			err := env.handler.Execute(testCase.action, &state, &message.Message{}, nil, testCase.origin)
			if err != nil {
				t.Fatalf("Execute(%q) error = %v", testCase.action, err)
			}

			if state != testCase.want {
				t.Errorf("state after %q = %+v, want %+v", testCase.action, state, testCase.want)
			}
		})
	}
}

func TestExecuteUnknownAction(t *testing.T) {
	t.Parallel()

	env := newActionEnv(t.TempDir())
	state := &domain.UserState{UserName: "Ana"}

	err := env.handler.Execute("launch_rocket", state, &message.Message{}, nil, "N")
	if err == nil || err.Error() != "unknown action: launch_rocket" {
		t.Errorf("Execute error = %v, want unknown action error", err)
	}

	if state.UserName != "Ana" {
		t.Errorf("state was modified: %+v", state)
	}
}

func TestExecuteSaveUserName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		existing string
		want     string
		wantErr  error
	}{
		{"plain name", "Ana", "", "Ana", nil},
		{"surrounding whitespace", "  Ana  ", "", "Ana", nil},
		{"full name is stored whole", "Ana María López", "", "Ana María López", nil},
		{"mi nombre es", "mi nombre es Ana", "", "Ana", nil},
		{"me llamo", "Me llamo Laura", "", "Laura", nil},
		{"llámame", "llámame Pepe", "", "Pepe", nil},
		{"prefix is matched case-insensitively", "MI NOMBRE ES Carla", "", "Carla", nil},
		{"empty message keeps the old name", "", "Old", "Old", nil},
		{"blank message keeps the old name", "   ", "Old", "Old", nil},
		// Doubt: a bare prefix is silently ignored (nil error), yet the flow still tells the user the name was updated.
		{"prefix without a name keeps the old name", "mi nombre es", "Old", "Old", nil},
		{"prefix with spaces only", "me llamo   ", "Old", "Old", nil},
		{"consonants only", "xyz", "Old", "Old", service.ErrInvalidName},
		{"too short", "Al", "Old", "Old", service.ErrInvalidName},
		{"too many words", "uno dos tres cuatro cinco", "Old", "Old", service.ErrInvalidName},
		{"invalid after the prefix", "me llamo xyz", "Old", "Old", service.ErrInvalidName},
		// Doubt: the prefix is only stripped from the start, so a sentence is stored verbatim
		// and the greeting later uses its first plausible word: "Hola".
		{"prefix in the middle of a sentence", "hola, me llamo Ana", "", "hola, me llamo Ana", nil},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			env := newActionEnv(t.TempDir())
			state := &domain.UserState{UserID: testUserID, UserName: testCase.existing}

			err := env.handler.Execute("save_user_name", state, &message.Message{Text: testCase.input}, nil, "N")
			if !errors.Is(err, testCase.wantErr) {
				t.Errorf("Execute error = %v, want %v", err, testCase.wantErr)
			}

			if state.UserName != testCase.want {
				t.Errorf("UserName = %q, want %q", state.UserName, testCase.want)
			}
		})
	}
}

func TestSaveVoucherStoresTheDownloadedImage(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "nested", "vouchers")
	env := newActionEnv(dir)
	state := &domain.UserState{UserID: testUserID}
	msg := &message.Message{PushName: "Ana Pérez", HasMedia: true, MediaType: "image"}

	err := env.handler.Execute("save_payment_voucher", state, msg, imageEvent("voucher"), "ENROLLMENT_PROCESS")
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}

	if filepath.Dir(state.VoucherPath) != dir {
		t.Errorf("VoucherPath = %q, want a file in %q", state.VoucherPath, dir)
	}

	// Doubt: the name is ASCII-only ("María José" loses its accented letters) and the
	// timestamp has one-second resolution, so two vouchers from one user in a second overwrite each other.
	if !regexp.MustCompile(`^51999000111_Ana_Prez_\d{10}\.jpeg$`).MatchString(filepath.Base(state.VoucherPath)) {
		t.Errorf("voucher file name = %q", filepath.Base(state.VoucherPath))
	}

	saved, err := os.ReadFile(filepath.Clean(state.VoucherPath))
	if err != nil || string(saved) != "jpeg-bytes" {
		t.Errorf("saved file = %q, %v; want the downloaded bytes", saved, err)
	}

	if env.wa.downloads != 1 {
		t.Errorf("downloads = %d, want 1", env.wa.downloads)
	}
}

func TestSaveVoucherFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name, userID, pushName, wantPrefix string
	}{
		{"phone is the part before the @", "51999@s.whatsapp.net", "Ana", "51999_Ana_"},
		{"user id without an @ is used whole", "51999", "Ana", "51999_Ana_"},
		{"spaces become underscores", "1@s.whatsapp.net", "Ana Maria", "1_Ana_Maria_"},
		{"hyphens and underscores survive", "1@s.whatsapp.net", "Ana-Maria_2", "1_Ana-Maria_2_"},
		{"accented letters are dropped", "1@s.whatsapp.net", "María José", "1_Mara_Jos_"},
		{"punctuation only falls back to user", "1@s.whatsapp.net", "!!!", "1_user_"},
		// Doubt: spaces are replaced before the fallback check, so a name of symbols and spaces yields "1___<ts>" instead of "user".
		{"symbols separated by a space do not fall back", "1@s.whatsapp.net", "!!! ???", "1___"},
		{"empty name falls back to user", "1@s.whatsapp.net", "", "1_user_"},
		{"path separators are removed", "1@s.whatsapp.net", "../etc", "1_etc_"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			env := newActionEnv(filepath.Join(t.TempDir(), "v"))
			state := &domain.UserState{UserID: testCase.userID}

			err := env.handler.Execute("save_payment_voucher", state, &message.Message{PushName: testCase.pushName}, imageEvent(""), "N")
			if err != nil {
				t.Fatalf("Execute error = %v", err)
			}

			base := filepath.Base(state.VoucherPath)
			if !strings.HasPrefix(base, testCase.wantPrefix) || !strings.HasSuffix(base, ".jpeg") {
				t.Errorf("file name = %q, want prefix %q and suffix .jpeg", base, testCase.wantPrefix)
			}
		})
	}
}

func TestSaveVoucherWithoutAnImageEscalatesAndReturnsNoError(t *testing.T) {
	t.Parallel()

	payloads := map[string]*events.Message{
		"a video":               mediaEvent("video", ""),
		"an empty payload":      newEvent(&waE2E.Message{}),
		"a text message":        textEvent("here you go"),
		"a document with image": mediaEvent("document", "photo"),
	}

	for name, evt := range payloads {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := newActionEnv(t.TempDir())
			state := &domain.UserState{UserID: testUserID}

			// Doubt: the flow still moves the user on to the "payment confirmed" node afterwards.
			err := env.handler.Execute("save_payment_voucher", state, &message.Message{}, evt, "N")
			if err != nil {
				t.Fatalf("Execute error = %v", err)
			}

			if !state.RequiresHumanAgent || state.VoucherPath != "" || env.wa.downloads != 0 {
				t.Errorf("state = %+v, downloads = %d; want escalation and no download", state, env.wa.downloads)
			}
		})
	}
}

func TestSaveVoucherRequiresARawMessageEvent(t *testing.T) {
	t.Parallel()

	env := newActionEnv(t.TempDir())

	for _, raw := range []any{nil, "not an event", &events.Connected{}} {
		err := env.handler.Execute("save_payment_voucher", &domain.UserState{}, &message.Message{}, raw, "N")
		if err == nil || !strings.Contains(err.Error(), "requires a raw message event") {
			t.Errorf("Execute(%T) error = %v, want a raw message event error", raw, err)
		}
	}
}

func TestSaveVoucherDownloadFailure(t *testing.T) {
	t.Parallel()

	env := newActionEnv(t.TempDir())
	env.wa.downloadErr = errors.New("connection reset")
	state := &domain.UserState{}

	err := env.handler.Execute("save_payment_voucher", state, &message.Message{}, imageEvent(""), "N")
	if !errors.Is(err, env.wa.downloadErr) || !strings.Contains(err.Error(), "could not download voucher") {
		t.Errorf("Execute error = %v, want a wrapped download error", err)
	}

	if state.VoucherPath != "" {
		t.Errorf("VoucherPath = %q, want it unset", state.VoucherPath)
	}
}

func TestSaveVoucherDirectoryFailure(t *testing.T) {
	t.Parallel()

	blocker := filepath.Join(t.TempDir(), "file")

	err := os.WriteFile(blocker, nil, 0o600)
	if err != nil {
		t.Fatal(err)
	}

	env := newActionEnv(filepath.Join(blocker, "vouchers"))
	state := &domain.UserState{}

	err = env.handler.Execute("save_payment_voucher", state, &message.Message{}, imageEvent(""), "N")
	if err == nil || !strings.Contains(err.Error(), "could not create voucher directory") {
		t.Errorf("Execute error = %v, want a directory error", err)
	}

	if state.VoucherPath != "" {
		t.Errorf("VoucherPath = %q, want it unset", state.VoucherPath)
	}
}

func TestSaveVoucherFileFailure(t *testing.T) {
	t.Parallel()

	// Permission bits do not stop root, so the failure cannot be provoked there.
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}

	dir := filepath.Join(t.TempDir(), "vouchers")

	err := os.Mkdir(dir, 0o500)
	if err != nil {
		t.Fatal(err)
	}

	env := newActionEnv(dir)
	state := &domain.UserState{}

	err = env.handler.Execute("save_payment_voucher", state, &message.Message{}, imageEvent(""), "N")
	if err == nil || !strings.Contains(err.Error(), "could not save voucher file") {
		t.Errorf("Execute error = %v, want a write error", err)
	}

	if state.VoucherPath != "" {
		t.Errorf("VoucherPath = %q, want it unset", state.VoucherPath)
	}
}
