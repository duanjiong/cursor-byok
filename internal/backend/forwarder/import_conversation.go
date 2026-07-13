package forwarder

import (
	"encoding/json"
	"fmt"
	"strings"

	"cursor/gen/agentv1"
	modeladapter "cursor/internal/backend/agent/model"
	promptengine "cursor/internal/backend/agent/prompt"
)

func (service *Service) importConversationState(item *ConversationFile, intent InboundIntent) ([]HistoryEntry, error) {
	state := intent.ConversationState
	if item == nil || state == nil {
		return nil, nil
	}
	item.TokenDetailsUsedTokens = state.GetTokenDetails().GetUsedTokens()

	messages, err := resolveImportedBootstrapMessages(state, intent)
	if err != nil {
		return nil, err
	}

	entries := make([]HistoryEntry, 0, len(messages)+2)
	for _, message := range messages {
		entry, ok, err := newModelMessageEntry(0, "", message)
		if err != nil {
			return nil, err
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	if len(entries) == 0 {
		summary, ok, err := importedConversationStateSummary(state)
		if err != nil {
			return nil, err
		}
		if ok {
			payload, err := json.Marshal(compactionSummaryEntryPayload{
				Summary: strings.TrimSpace(summary),
				Trigger: "imported_conversation_state",
			})
			if err != nil {
				return nil, fmt.Errorf("encode imported summary context: %w", err)
			}
			entries = append(entries, HistoryEntry{
				TurnSeq: 0,
				Role:    "system",
				Kind:    "compacted_summary",
				Payload: payload,
			})
		}
	}
	runtimeState, ok, err := runtimeStatePayloadFromConversationState(state)
	if err != nil {
		return nil, err
	}
	if ok {
		payload, err := json.Marshal(runtimeState)
		if err != nil {
			return nil, fmt.Errorf("encode imported runtime state context: %w", err)
		}
		entries = append(entries, HistoryEntry{
			TurnSeq: 0,
			Role:    "system",
			Kind:    "runtime_state",
			Payload: payload,
		})
	}
	return entries, nil
}

func resolveImportedBootstrapMessages(state *agentv1.ConversationStateStructure, intent InboundIntent) ([]modeladapter.Message, error) {
	if history := intent.ConversationHistory; history != nil && len(history.GetMessages()) > 0 {
		return importedConversationHistoryModelMessages(history)
	}
	messages, err := importedConversationStateModelMessages(state)
	if err != nil {
		return nil, err
	}
	if len(intent.PrependUserMessages) > 0 {
		messages = truncateImportedReplayByPrepend(messages, intent.PrependUserMessages)
	}
	return messages, nil
}

func importedConversationHistoryModelMessages(history *agentv1.ConversationHistory) ([]modeladapter.Message, error) {
	if history == nil {
		return nil, nil
	}
	messages := make([]modeladapter.Message, 0, len(history.GetMessages()))
	for _, item := range history.GetMessages() {
		if item == nil {
			continue
		}
		switch typed := item.GetMessage().(type) {
		case *agentv1.ConversationHistoryMessage_User:
			text := conversationHistoryUserText(typed.User)
			if text == "" {
				continue
			}
			replay, ok := promptengine.BuildUserQueryReplayMessage(text)
			if !ok {
				continue
			}
			messages = append(messages, toModelMessage(replay))
		case *agentv1.ConversationHistoryMessage_Assistant:
			message := modeladapter.Message{Role: "assistant"}
			for _, content := range typed.Assistant.GetContent() {
				if content == nil {
					continue
				}
				switch part := content.GetContent().(type) {
				case *agentv1.ConversationHistoryAssistantContent_Text:
					message.Content += part.Text.GetText()
				case *agentv1.ConversationHistoryAssistantContent_Reasoning:
					message.ReasoningContent += part.Reasoning.GetText()
					if signature := strings.TrimSpace(part.Reasoning.GetSignature()); signature != "" {
						message.ReasoningSignature = signature
					}
				case *agentv1.ConversationHistoryAssistantContent_ToolCall:
					message.ToolCalls = append(message.ToolCalls, modeladapter.ToolCallDescriptor{
						ID:   strings.TrimSpace(part.ToolCall.GetToolCallId()),
						Type: "function",
						Function: modeladapter.ToolCallFunctionShape{
							Name:      strings.TrimSpace(part.ToolCall.GetToolName()),
							Arguments: part.ToolCall.GetArgsJson(),
						},
					})
				}
			}
			messages = append(messages, message)
		case *agentv1.ConversationHistoryMessage_Tool:
			messages = append(messages, modeladapter.Message{
				Role:       "tool",
				Content:    conversationHistoryToolText(typed.Tool),
				ToolCallID: strings.TrimSpace(typed.Tool.GetToolCallId()),
				Name:       strings.TrimSpace(typed.Tool.GetToolName()),
			})
		}
	}
	return normalizeReplayMessageSequence(messages), nil
}

func conversationHistoryUserText(message *agentv1.ConversationHistoryUserMessage) string {
	if message == nil {
		return ""
	}
	parts := make([]string, 0, len(message.GetContent()))
	for _, content := range message.GetContent() {
		if content == nil {
			continue
		}
		if text := content.GetText(); text != nil {
			if trimmed := strings.TrimSpace(text.GetText()); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func conversationHistoryToolText(message *agentv1.ConversationHistoryToolMessage) string {
	if message == nil {
		return ""
	}
	parts := make([]string, 0, len(message.GetContent()))
	for _, content := range message.GetContent() {
		if content == nil {
			continue
		}
		if text := content.GetText(); text != nil {
			if trimmed := strings.TrimSpace(text.GetText()); trimmed != "" {
				parts = append(parts, trimmed)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func truncateImportedReplayByPrepend(messages []modeladapter.Message, prepend []*agentv1.UserMessage) []modeladapter.Message {
	prependTexts := prependUserMessageTexts(prepend)
	if len(prependTexts) == 0 || len(messages) == 0 {
		return messages
	}
	firstIdx, lastUserIdx, ok := matchPrependToReplayUserMessages(messages, prependTexts)
	if !ok {
		return messages
	}
	endIdx := lastUserIdx
	for index := lastUserIdx + 1; index < len(messages); index++ {
		if isReplayUserRole(messages[index].Role) {
			break
		}
		endIdx = index
	}
	return append([]modeladapter.Message(nil), messages[firstIdx:endIdx+1]...)
}

func prependUserMessageTexts(messages []*agentv1.UserMessage) []string {
	if len(messages) == 0 {
		return nil
	}
	texts := make([]string, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if text := strings.TrimSpace(message.GetText()); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

func matchPrependToReplayUserMessages(messages []modeladapter.Message, prependTexts []string) (int, int, bool) {
	userSlots := make([]struct {
		index int
		text  string
	}, 0, len(messages))
	for index, message := range messages {
		if !isReplayUserRole(message.Role) {
			continue
		}
		text := replayMessagePlainUserText(message)
		if text == "" {
			continue
		}
		userSlots = append(userSlots, struct {
			index int
			text  string
		}{index: index, text: text})
	}
	if len(userSlots) == 0 {
		return 0, 0, false
	}
	slotIndex := 0
	firstIdx := -1
	lastIdx := -1
	for _, prependText := range prependTexts {
		matched := false
		for candidate := slotIndex; candidate < len(userSlots); candidate++ {
			if !replayUserTextsEquivalent(userSlots[candidate].text, prependText) {
				continue
			}
			if firstIdx < 0 {
				firstIdx = userSlots[candidate].index
			}
			lastIdx = userSlots[candidate].index
			slotIndex = candidate + 1
			matched = true
			break
		}
		if !matched {
			return 0, 0, false
		}
	}
	if firstIdx < 0 || lastIdx < 0 {
		return 0, 0, false
	}
	return firstIdx, lastIdx, true
}

func replayUserTextsEquivalent(replayText string, prependText string) bool {
	replayText = strings.TrimSpace(replayText)
	prependText = strings.TrimSpace(prependText)
	if replayText == "" || prependText == "" {
		return false
	}
	return replayText == prependText ||
		strings.Contains(replayText, prependText) ||
		strings.Contains(prependText, replayText)
}

func replayMessagePlainUserText(message modeladapter.Message) string {
	if !isReplayUserRole(message.Role) {
		return ""
	}
	text := message.Content
	if strings.TrimSpace(text) == "" && len(message.ContentParts) > 0 {
		text = collapseModelMessageTextParts(message.ContentParts)
	}
	if tagged := textBetweenReplayTag(text, "current_user_request"); tagged != "" {
		return tagged
	}
	if tagged := textBetweenReplayTag(text, "user_query"); tagged != "" {
		return tagged
	}
	return strings.TrimSpace(text)
}

func isReplayUserRole(role string) bool {
	return strings.EqualFold(strings.TrimSpace(role), "user")
}

func textBetweenReplayTag(text string, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	start := strings.LastIndex(text, openTag)
	if start < 0 {
		return ""
	}
	start += len(openTag)
	end := strings.Index(text[start:], closeTag)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(text[start : start+end])
}

func collapseModelMessageTextParts(parts []modeladapter.ContentPart) string {
	if len(parts) == 0 {
		return ""
	}
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part.Text); trimmed != "" {
			segments = append(segments, trimmed)
		}
	}
	return strings.Join(segments, "\n")
}
