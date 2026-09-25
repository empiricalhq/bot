package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"whatsbot/internal/domain"
	"whatsbot/internal/message"
	"whatsbot/pkg/utils"
)

type FSM interface {
	DetermineNext(state *domain.UserState, msg *message.Message) (nodeID, action string)
	GetNode(nodeID string) *domain.Node
	GetStartNode() string
}

type fsm struct {
	flow       *domain.Flow
	regexCache map[string]*regexp.Regexp
	mutex      sync.RWMutex
	logger     *slog.Logger
}

func NewFSM(flow *domain.Flow, logger *slog.Logger) FSM {
	return &fsm{
		flow:       flow,
		regexCache: make(map[string]*regexp.Regexp),
		logger:     logger,
	}
}

func (f *fsm) DetermineNext(state *domain.UserState, msg *message.Message) (nodeID, action string) {
	input := strings.ToLower(strings.TrimSpace(msg.Text))

	currentNode, nodeExists := f.flow.Nodes[state.CurrentNode]
	if !nodeExists {
		// User is in a node that no longer exists => reset to start.
		f.logger.Error("Current node in user state does not exist in flow", "node", state.CurrentNode)

		return f.flow.StartNode, ""
	}

	// 1. Check node-specific transitions first (context takes priority).
	// The action returned is the transition's own; the action of the node being entered
	// is the caller's to run, so it also runs when a global transition or a fallback leads there.
	// If message has media, test media rules before text rules
	// so a caption doesn't accidentally match a keyword: (image with caption: "listo")
	for _, transition := range transitionsInOrder(currentNode.Transitions, msg.HasMedia) {
		if f.matchesCondition(input, msg, transition.Condition) {
			f.logger.Debug("Matched node transition",
				"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", transition.Action, "condition_type", transition.Condition.Type)

			return transition.Target, transition.Action
		}
	}

	// 2. If no node transition matched, check global transitions.
	// Normally skipped if node ignores globals, except for help requests
	// (e.g., user types "ayuda" => always allowed).
	if transition, ok := f.matchGlobal(input, msg, currentNode.IgnoreGlobalTransitions); ok {
		f.logger.Debug("Matched global transition",
			"user", state.UserID, "from_node", state.CurrentNode, "to_node", transition.Target, "action", transition.Action, "condition_type", transition.Condition.Type)

		return transition.Target, transition.Action
	}

	// 3. If no transition matched, handle special fallback cases.
	// Example: user sends a video where an image (voucher) was expected.
	expectsMedia := slices.ContainsFunc(currentNode.Transitions, func(t domain.Transition) bool { return t.Condition.Type == "media_type" })
	if msg.HasMedia && expectsMedia {
		f.logger.Debug("Matched specific fallback for wrong media type",
			"user", state.UserID, "from_node", state.CurrentNode, "sent_media", msg.MediaType)

		return state.CurrentNode, actionTriggerFallbackWrongMedia
	}

	// 4. Nothing matched => remain in current node and trigger generic fallback response.
	f.logger.Debug("No transition matched, staying in current node and triggering fallback response",
		"user", state.UserID, "from_node", state.CurrentNode)

	// Special action tells caller to send fallback message but keep state unchanged.
	return state.CurrentNode, actionTriggerFallbackResponse
}

// transitionsInOrder lists the transitions in the order they are tried. For a message with media,
// the media conditions come first, then the rest (e.g. a caption), each group in flow order.
func transitionsInOrder(transitions []domain.Transition, hasMedia bool) []domain.Transition {
	if !hasMedia {
		return transitions
	}

	ordered := make([]domain.Transition, 0, len(transitions))

	for _, t := range transitions {
		if isMediaCondition(t.Condition) {
			ordered = append(ordered, t)
		}
	}

	for _, t := range transitions {
		if !isMediaCondition(t.Condition) {
			ordered = append(ordered, t)
		}
	}

	return ordered
}

func isMediaCondition(condition domain.Condition) bool {
	return condition.Type == "media" || condition.Type == "media_type"
}

func (f *fsm) GetStartNode() string {
	return f.flow.StartNode
}

func (f *fsm) GetNode(nodeID string) *domain.Node {
	node, exists := f.flow.Nodes[nodeID]
	if !exists {
		return nil
	}

	return &node
}

// matchGlobal finds the first global transition that applies to the message.
// A node that ignores globals still allows the help transition.
func (f *fsm) matchGlobal(input string, msg *message.Message, ignoreGlobals bool) (domain.Transition, bool) {
	for _, transition := range f.flow.GlobalTransitions {
		isHelpTransition := transition.Target == "NEEDS_ASSISTANCE"

		if (!ignoreGlobals || isHelpTransition) && f.matchesCondition(input, msg, transition.Condition) {
			return transition, true
		}
	}

	return domain.Transition{}, false
}

func (f *fsm) matchesCondition(input string, msg *message.Message, condition domain.Condition) bool {
	switch condition.Type {
	case "exact":
		// Match only if input equals one of the values (case-insensitive).
		return slices.ContainsFunc(condition.Value, func(value string) bool { return strings.EqualFold(input, value) })

	case "keyword":
		return matchesKeyword(input, condition.Value)

	case "regex":
		return f.matchesRegex(input, condition.Regex)

	case "any_text":
		return input != "" && !msg.HasMedia

	case "media":
		return msg.HasMedia

	case "media_type":
		return slices.Contains(condition.Value, msg.MediaType)
	}

	return false
}

// maxExactLetters is the length up to which a word must match a keyword exactly:
// one edit turns "hora" into "hola", so a typo tolerance on short words selects the wrong option.
const maxExactLetters = 4

// maxTypoEdits is the number of edits tolerated in a word longer than maxExactLetters.
const maxTypoEdits = 2

var accentFolder = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")

// matchesKeyword reports whether the text contains one of the keywords as whole words,
// ignoring case and accents. A single-word keyword also matches a misspelling of it.
func matchesKeyword(text string, keywords []string) bool {
	words := wordsOf(text)

	return slices.ContainsFunc(keywords, func(keyword string) bool {
		phrase := wordsOf(keyword)
		if len(phrase) == 0 {
			// Nothing to split on (an empty keyword, or only symbols): compare the raw text.
			return strings.Contains(accentFolder.Replace(strings.ToLower(text)), accentFolder.Replace(strings.ToLower(keyword)))
		}

		return containsPhrase(words, phrase)
	})
}

// containsPhrase reports whether the words include the phrase as consecutive words.
// A phrase of one word also matches a misspelling; a longer one must match exactly.
func containsPhrase(words, phrase []string) bool {
	if len(phrase) == 1 {
		return slices.ContainsFunc(words, func(word string) bool { return isSameWord(word, phrase[0]) })
	}

	for start := 0; start+len(phrase) <= len(words); start++ {
		if slices.Equal(words[start:start+len(phrase)], phrase) {
			return true
		}
	}

	return false
}

// wordsOf splits text into lowercase, accent-free words made of letters and digits.
func wordsOf(text string) []string {
	return strings.FieldsFunc(accentFolder.Replace(strings.ToLower(text)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// isSameWord reports whether word is keyword or a typo of it. Lengths count letters, not bytes.
func isSameWord(word, keyword string) bool {
	if min(utf8.RuneCountInString(word), utf8.RuneCountInString(keyword)) <= maxExactLetters {
		return word == keyword
	}

	return utils.LevenshteinDistance(word, keyword) <= maxTypoEdits
}

func (f *fsm) matchesRegex(input, pattern string) bool {
	if pattern == "" {
		return false
	}

	f.mutex.RLock()
	regex, exists := f.regexCache[pattern]
	f.mutex.RUnlock()

	if !exists {
		f.mutex.Lock()

		// Double-check after acquiring write lock
		var err error
		if regex, exists = f.regexCache[pattern]; !exists {
			// The input is lowercased, so the pattern must not be case-sensitive either.
			regex, err = regexp.Compile("(?i)" + pattern)
			if err != nil {
				f.logger.Error("Invalid regex pattern", "pattern", pattern, "error", err)
				f.mutex.Unlock()

				return false
			}

			f.regexCache[pattern] = regex
		}

		f.mutex.Unlock()
	}

	return regex.MatchString(input)
}

func LoadFlow(path string) (*domain.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read flow file: %w", err)
	}

	var flow domain.Flow

	err = json.Unmarshal(data, &flow)
	if err != nil {
		return nil, fmt.Errorf("failed to parse flow file: %w", err)
	}

	err = processTransitionIncludes(&flow)
	if err != nil {
		return nil, fmt.Errorf("failed to process transition includes: %w", err)
	}

	return &flow, validateFlow(&flow)
}

// processTransitionIncludes merges shared transitions from TransitionGroups into nodes.
func processTransitionIncludes(flow *domain.Flow) error {
	if len(flow.TransitionGroups) == 0 {
		return nil
	}

	processedNodes := make(map[string]domain.Node, len(flow.Nodes))
	for nodeID, node := range flow.Nodes {
		if node.IncludeTransitions != "" {
			groupName := node.IncludeTransitions

			group, ok := flow.TransitionGroups[groupName]
			if !ok {
				return fmt.Errorf("node %q includes non-existent transition group %q", nodeID, groupName)
			}

			// Prepend the group transitions so that node-specific transitions are checked last.
			node.Transitions = append(group, node.Transitions...)
		}

		processedNodes[nodeID] = node
	}

	flow.Nodes = processedNodes

	return nil
}

func validateFlow(flow *domain.Flow) error {
	if flow.StartNode == "" {
		return errors.New("start_node cannot be empty")
	}

	_, exists := flow.Nodes[flow.StartNode]
	if !exists {
		return errors.New("start_node not found in nodes")
	}

	// Helper to check that all transitions in a slice point to existing nodes.
	checkTransitions := func(transitions []domain.Transition, source string) error {
		for _, transition := range transitions {
			if _, exists := flow.Nodes[transition.Target]; !exists {
				return fmt.Errorf("invalid transition from %s to non-existent node %q", source, transition.Target)
			}
		}

		return nil
	}

	// Validate global transitions.
	err := checkTransitions(flow.GlobalTransitions, "global_transitions")
	if err != nil {
		return err
	}

	// Validate transitions for each node.
	for nodeID, node := range flow.Nodes {
		source := fmt.Sprintf("node %q", nodeID)

		err = checkTransitions(node.Transitions, source)
		if err != nil {
			return err
		}
	}

	return nil
}
