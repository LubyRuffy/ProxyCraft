package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
)

type LLMExtracted struct {
	Provider  string           `json:"provider,omitempty"`
	Model     string           `json:"model,omitempty"`
	Streaming bool             `json:"streaming,omitempty"`
	Request   *LLMRequestInfo  `json:"request,omitempty"`
	Response  *LLMResponseInfo `json:"response,omitempty"`
}

type LLMRequestInfo struct {
	Prompt    string      `json:"prompt,omitempty"`
	ToolCalls interface{} `json:"toolCalls,omitempty"`
	Tools     interface{} `json:"tools,omitempty"`
}

type LLMResponseInfo struct {
	Content   string      `json:"content,omitempty"`
	ToolCalls interface{} `json:"toolCalls,omitempty"`
	Reasoning string      `json:"reasoning,omitempty"`
}

// ExtractLLM extracts structured LLM request/response data.
func ExtractLLM(entry *handlers.TrafficEntry, includeRequest bool, includeResponse bool) *LLMExtracted {
	if entry == nil {
		return nil
	}

	reqPayload := parseJSONMap(entry.RequestBody)
	provider := detectLLMProvider(entry, reqPayload)
	if provider == "" {
		return nil
	}

	result := &LLMExtracted{
		Provider:  provider,
		Streaming: entry.IsSSE,
	}

	if streamVal, ok := asBool(reqPayload, "stream"); ok {
		result.Streaming = streamVal || result.Streaming
	}

	if model := asStringField(reqPayload, "model"); model != "" {
		result.Model = model
	}

	if includeRequest && reqPayload != nil {
		if req := extractLLMRequest(reqPayload, provider); req != nil {
			result.Request = req
		}
	}

	if includeResponse {
		resp, model := extractLLMResponse(entry, provider)
		if resp != nil {
			result.Response = resp
		}
		if result.Model == "" && model != "" {
			result.Model = model
		}
	}

	if result.Request == nil && result.Response == nil {
		if includeResponse && !includeRequest {
			return result
		}
		return nil
	}

	return result
}

func parseJSONMap(body []byte) map[string]interface{} {
	if len(body) == 0 {
		return nil
	}
	var payload interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	m, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

func detectLLMProvider(entry *handlers.TrafficEntry, payload map[string]interface{}) string {
	host := strings.ToLower(entry.Host)
	path := strings.ToLower(entry.Path)
	url := strings.ToLower(entry.URL)

	if strings.Contains(host, "openai") || strings.Contains(url, "openai") {
		return "openai"
	}
	if strings.Contains(host, "anthropic") || strings.Contains(host, "claude") || strings.Contains(path, "/v1/messages") || strings.Contains(path, "/v1/complete") {
		return "claude"
	}
	if strings.Contains(host, "generativelanguage") || strings.Contains(path, "generatecontent") || strings.Contains(path, "streamgeneratecontent") {
		return "gemini"
	}
	if strings.Contains(host, "ollama") || strings.Contains(path, "/api/generate") || strings.Contains(path, "/api/chat") {
		return "ollama"
	}
	if strings.Contains(path, "/responses") {
		return "openai-compatible"
	}
	if strings.Contains(path, "/v1/chat/completions") || strings.Contains(path, "/v1/completions") || strings.Contains(path, "/v1/responses") {
		return "openai-compatible"
	}

	if payload != nil {
		if _, ok := payload["messages"]; ok {
			return "openai-compatible"
		}
		if _, ok := payload["prompt"]; ok {
			return "openai-compatible"
		}
		if _, ok := payload["input"]; ok {
			return "openai-compatible"
		}
		if _, ok := payload["contents"]; ok {
			return "gemini"
		}
	}

	return ""
}

func extractLLMRequest(payload map[string]interface{}, provider string) *LLMRequestInfo {
	var prompt string
	var toolCalls interface{}
	var tools interface{}

	switch provider {
	case "gemini":
		prompt = buildPromptFromContents(asSlice(payload["contents"]))
	default:
		prompt = buildPromptFromMessages(asSlice(payload["messages"]))
		if prompt == "" {
			prompt = buildPromptFromInput(payload["input"])
		}
		if prompt == "" {
			prompt = buildPromptFromPrompt(payload["prompt"])
		}
		if provider == "claude" {
			prompt = mergePromptWithSystem(prompt, asStringField(payload, "system"))
		}
	}

	if provider == "claude" && prompt == "" {
		prompt = mergePromptWithSystem(prompt, asStringField(payload, "system"))
	}

	tools = payload["tools"]
	if tools == nil {
		tools = payload["functions"]
	}

	toolCalls = extractToolCallsFromRequest(payload)

	if prompt == "" && tools == nil && toolCalls == nil {
		return nil
	}

	return &LLMRequestInfo{
		Prompt:    prompt,
		ToolCalls: toolCalls,
		Tools:     tools,
	}
}

func extractLLMResponse(entry *handlers.TrafficEntry, provider string) (*LLMResponseInfo, string) {
	if len(entry.ResponseBody) == 0 {
		return nil, ""
	}

	contentType := strings.ToLower(entry.ContentType)
	if entry.IsSSE || strings.Contains(contentType, "text/event-stream") {
		content, reasoning, toolCalls, model := extractLLMResponseFromSSE(entry.ResponseBody, provider)
		if content == "" && reasoning == "" && toolCalls == nil {
			return nil, model
		}
		return &LLMResponseInfo{
			Content:   content,
			Reasoning: reasoning,
			ToolCalls: toolCalls,
		}, model
	}

	payload := parseJSONMap(entry.ResponseBody)
	if payload == nil {
		return nil, ""
	}

	content, reasoning, toolCalls, model := extractLLMResponseFromJSON(payload, provider)
	if content == "" && reasoning == "" && toolCalls == nil {
		return nil, model
	}

	return &LLMResponseInfo{
		Content:   content,
		Reasoning: reasoning,
		ToolCalls: toolCalls,
	}, model
}

func extractLLMResponseFromJSON(payload map[string]interface{}, provider string) (string, string, interface{}, string) {
	var content strings.Builder
	var reasoning strings.Builder
	var toolCalls []interface{}
	model := asStringField(payload, "model")

	if outputText, ok := payload["output_text"].(string); ok && outputText != "" {
		content.WriteString(outputText)
	}

	if outputItems := asSlice(payload["output"]); len(outputItems) > 0 {
		for _, item := range outputItems {
			itemMap := asMap(item)
			if itemMap == nil {
				continue
			}
			itemType := asStringField(itemMap, "type")
			if itemType == "reasoning" {
				if text := extractTextFromContent(itemMap["content"]); text != "" {
					reasoning.WriteString(text)
				}
				continue
			}
			if itemType == "message" {
				if text := extractTextFromContent(itemMap["content"]); text != "" {
					content.WriteString(text)
				}
			} else if itemType == "function_call" || itemType == "tool_call" || itemType == "tool_use" {
				if call := buildToolCallFromOutputItem(itemMap); call != nil {
					toolCalls = append(toolCalls, call)
				}
			}
		}
	}

	if choices := asSlice(payload["choices"]); len(choices) > 0 {
		first := asMap(choices[0])
		if first != nil {
			if message := asMap(first["message"]); message != nil {
				if text := extractTextFromContent(message["content"]); text != "" {
					content.WriteString(text)
				}
				if reasoningText := extractReasoningFromMap(message); reasoningText != "" {
					reasoning.WriteString(reasoningText)
				}
				if calls := normalizeToolCalls(message["tool_calls"]); len(calls) > 0 {
					toolCalls = append(toolCalls, calls...)
				}
				if functionCall := asMap(message["function_call"]); functionCall != nil {
					toolCalls = append(toolCalls, wrapFunctionCall(functionCall))
				}
			}
			if content.Len() == 0 {
				if text := asStringField(first, "text"); text != "" {
					content.WriteString(text)
				}
			}
		}
	}

	if content.Len() == 0 && provider == "ollama" {
		if text := asStringField(payload, "response"); text != "" {
			content.WriteString(text)
		}
		if message := asMap(payload["message"]); message != nil {
			if text := extractTextFromContent(message["content"]); text != "" {
				content.WriteString(text)
			}
			if calls := normalizeToolCalls(message["tool_calls"]); len(calls) > 0 {
				toolCalls = append(toolCalls, calls...)
			}
		}
	}

	if provider == "claude" {
		if claudeContent := asSlice(payload["content"]); len(claudeContent) > 0 {
			claudeText, claudeReasoning, claudeToolCalls := parseClaudeContentBlocks(claudeContent)
			if claudeText != "" {
				content.WriteString(claudeText)
			}
			if claudeReasoning != "" {
				reasoning.WriteString(claudeReasoning)
			}
			if len(claudeToolCalls) > 0 && len(toolCalls) == 0 {
				toolCalls = append(toolCalls, claudeToolCalls...)
			}
		}
		if reasoning.Len() == 0 {
			if text := extractReasoningFromMap(payload); text != "" {
				reasoning.WriteString(text)
			}
		}
	}

	if provider == "gemini" {
		if candidates := asSlice(payload["candidates"]); len(candidates) > 0 {
			candidate := asMap(candidates[0])
			if candidate != nil {
				if contentMap := asMap(candidate["content"]); contentMap != nil {
					parts := asSlice(contentMap["parts"])
					partText, partReasoning, partToolCalls := parseGeminiParts(parts)
					if partText != "" {
						content.WriteString(partText)
					}
					if partReasoning != "" {
						reasoning.WriteString(partReasoning)
					}
					if len(partToolCalls) > 0 && len(toolCalls) == 0 {
						toolCalls = append(toolCalls, partToolCalls...)
					}
				}
			}
		}
	}

	if reasoning.Len() == 0 {
		if text := extractReasoningFromMap(payload); text != "" {
			reasoning.WriteString(text)
		}
	}

	var toolCallsResult interface{}
	if len(toolCalls) > 0 {
		toolCallsResult = toolCalls
	}

	return strings.TrimSpace(content.String()), strings.TrimSpace(reasoning.String()), toolCallsResult, model
}

func extractLLMResponseFromSSE(body []byte, provider string) (string, string, interface{}, string) {
	events := parseSSEDataEvents(body)
	var content strings.Builder
	var reasoning strings.Builder
	var toolCalls []interface{}
	var openaiToolCalls []map[string]interface{}
	responseState := &openAIResponseStreamState{}
	model := ""

	for _, data := range events {
		data = strings.TrimSpace(data)
		if data == "" || strings.EqualFold(data, "[done]") {
			continue
		}

		var payload interface{}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			continue
		}
		payloadMap := asMap(payload)
		if payloadMap == nil {
			continue
		}

		if model == "" {
			model = asStringField(payloadMap, "model")
			if model == "" {
				if responseMap := asMap(payloadMap["response"]); responseMap != nil {
					model = asStringField(responseMap, "model")
				}
			}
		}

		switch provider {
		case "claude":
			parseClaudeSSEPayload(payloadMap, &content, &reasoning, &toolCalls)
		case "gemini":
			parseGeminiSSEPayload(payloadMap, &content, &reasoning, &toolCalls)
		case "ollama":
			parseOllamaSSEPayload(payloadMap, &content, &toolCalls)
		default:
			if parseOpenAIResponsesSSEPayload(payloadMap, &content, &reasoning, responseState, &model) {
				continue
			}
			parseOpenAISSEPayload(payloadMap, &content, &reasoning, &openaiToolCalls)
		}
	}

	if len(openaiToolCalls) > 0 {
		for _, call := range openaiToolCalls {
			toolCalls = append(toolCalls, call)
		}
	}
	if len(responseState.toolCalls) > 0 && len(toolCalls) == 0 {
		toolCalls = append(toolCalls, responseState.toolCalls...)
	}

	return strings.TrimSpace(content.String()), strings.TrimSpace(reasoning.String()), toolCalls, model
}

func parseSSEDataEvents(body []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var dataLines []string
	var events []string
	var dataPerLine []string
	hasEmptyLine := false

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			hasEmptyLine = true
			if len(dataLines) > 0 {
				events = append(events, strings.Join(dataLines, "\n"))
				dataLines = nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			dataLines = append(dataLines, data)
			dataPerLine = append(dataPerLine, data)
		}
	}

	if len(dataLines) > 0 {
		events = append(events, strings.Join(dataLines, "\n"))
	}

	if !hasEmptyLine && len(dataPerLine) > 1 {
		return dataPerLine
	}

	return events
}

func parseOpenAISSEPayload(payload map[string]interface{}, content *strings.Builder, reasoning *strings.Builder, toolCalls *[]map[string]interface{}) {
	if choices := asSlice(payload["choices"]); len(choices) > 0 {
		choice := asMap(choices[0])
		if choice == nil {
			return
		}
		delta := asMap(choice["delta"])
		if delta == nil {
			return
		}
		if text := extractTextFromContentRaw(delta["content"]); text != "" {
			content.WriteString(text)
		}
		if reasoningText := extractReasoningFromMap(delta); reasoningText != "" {
			reasoning.WriteString(reasoningText)
		}
		if deltaToolCalls := asSlice(delta["tool_calls"]); len(deltaToolCalls) > 0 {
			*toolCalls = mergeOpenAIToolCalls(*toolCalls, deltaToolCalls)
		}
		if functionCall := asMap(delta["function_call"]); functionCall != nil {
			*toolCalls = mergeOpenAIToolCalls(*toolCalls, []interface{}{
				map[string]interface{}{
					"index":    float64(len(*toolCalls)),
					"type":     "function",
					"function": functionCall,
				},
			})
		}
	}
}

type openAIResponseStreamState struct {
	seenTextDelta      bool
	seenReasoningDelta bool
	seenTextItems      map[string]bool
	seenReasoningItems map[string]bool
	seenTextParts      map[string]bool
	seenReasoningParts map[string]bool
	toolCalls          []interface{}
}

func (state *openAIResponseStreamState) markTextSeen(itemID string, contentIndex interface{}) {
	if state == nil {
		return
	}
	state.seenTextDelta = true
	if itemID == "" {
		return
	}
	if state.seenTextItems == nil {
		state.seenTextItems = make(map[string]bool)
	}
	state.seenTextItems[itemID] = true
	key := buildContentIndexKey(itemID, contentIndex)
	if key == "" {
		return
	}
	if state.seenTextParts == nil {
		state.seenTextParts = make(map[string]bool)
	}
	state.seenTextParts[key] = true
}

func (state *openAIResponseStreamState) markReasoningSeen(itemID string, contentIndex interface{}) {
	if state == nil {
		return
	}
	state.seenReasoningDelta = true
	if itemID == "" {
		return
	}
	if state.seenReasoningItems == nil {
		state.seenReasoningItems = make(map[string]bool)
	}
	state.seenReasoningItems[itemID] = true
	key := buildContentIndexKey(itemID, contentIndex)
	if key == "" {
		return
	}
	if state.seenReasoningParts == nil {
		state.seenReasoningParts = make(map[string]bool)
	}
	state.seenReasoningParts[key] = true
}

func (state *openAIResponseStreamState) hasSeenText(itemID string, contentIndex interface{}) bool {
	if state == nil {
		return false
	}
	if itemID != "" {
		if contentIndex != nil {
			key := buildContentIndexKey(itemID, contentIndex)
			if key != "" && state.seenTextParts != nil && state.seenTextParts[key] {
				return true
			}
		}
		if state.seenTextItems != nil && state.seenTextItems[itemID] {
			return true
		}
	}
	return state.seenTextDelta
}

func (state *openAIResponseStreamState) hasSeenReasoning(itemID string, contentIndex interface{}) bool {
	if state == nil {
		return false
	}
	if itemID != "" {
		if contentIndex != nil {
			key := buildContentIndexKey(itemID, contentIndex)
			if key != "" && state.seenReasoningParts != nil && state.seenReasoningParts[key] {
				return true
			}
		}
		if state.seenReasoningItems != nil && state.seenReasoningItems[itemID] {
			return true
		}
	}
	return state.seenReasoningDelta
}

func parseOpenAIResponsesSSEPayload(payload map[string]interface{}, content *strings.Builder, reasoning *strings.Builder, state *openAIResponseStreamState, model *string) bool {
	if state == nil {
		return false
	}
	payloadType := asStringField(payload, "type")
	if !strings.HasPrefix(payloadType, "response.") {
		return false
	}
	itemID := asString(payload["item_id"])
	contentIndex := payload["content_index"]

	if model != nil && *model == "" {
		if responseMap := asMap(payload["response"]); responseMap != nil {
			if m := asStringField(responseMap, "model"); m != "" {
				*model = m
			}
		}
	}

	if delta, ok := payload["delta"].(string); ok && delta != "" {
		if strings.Contains(payloadType, "output_text") {
			content.WriteString(delta)
			state.markTextSeen(itemID, contentIndex)
			return true
		}
		if strings.Contains(payloadType, "reasoning") {
			reasoning.WriteString(delta)
			state.markReasoningSeen(itemID, contentIndex)
			return true
		}
	}

	if text, ok := payload["text"].(string); ok && text != "" {
		if strings.Contains(payloadType, "output_text") && !state.hasSeenText(itemID, contentIndex) {
			content.WriteString(text)
			state.markTextSeen(itemID, contentIndex)
			return true
		}
		if strings.Contains(payloadType, "reasoning") && !state.hasSeenReasoning(itemID, contentIndex) {
			reasoning.WriteString(text)
			state.markReasoningSeen(itemID, contentIndex)
			return true
		}
	}

	if deltaMap := asMap(payload["delta"]); deltaMap != nil {
		if contentVal, ok := deltaMap["content"]; ok {
			contentText, reasoningText, toolCalls := parseOpenAIResponseContentParts(contentVal)
			if contentText != "" && !state.hasSeenText(itemID, contentIndex) {
				content.WriteString(contentText)
				state.markTextSeen(itemID, contentIndex)
			}
			if reasoningText != "" && !state.hasSeenReasoning(itemID, contentIndex) {
				reasoning.WriteString(reasoningText)
				state.markReasoningSeen(itemID, contentIndex)
			}
			if len(toolCalls) > 0 {
				appendToolCalls(state, toolCalls)
			}
			return true
		}
	}

	if partVal, ok := payload["part"]; ok {
		contentText, reasoningText, toolCalls := parseOpenAIResponseContentParts([]interface{}{partVal})
		if contentText != "" && !state.hasSeenText(itemID, contentIndex) {
			content.WriteString(contentText)
			state.markTextSeen(itemID, contentIndex)
		}
		if reasoningText != "" && !state.hasSeenReasoning(itemID, contentIndex) {
			reasoning.WriteString(reasoningText)
			state.markReasoningSeen(itemID, contentIndex)
		}
		if len(toolCalls) > 0 {
			appendToolCalls(state, toolCalls)
		}
		return true
	}

	if itemMap := asMap(payload["item"]); itemMap != nil {
		handleOpenAIOutputItem(itemMap, content, reasoning, state)
	}
	if itemMap := asMap(payload["output_item"]); itemMap != nil {
		handleOpenAIOutputItem(itemMap, content, reasoning, state)
	}
	if outputItems := asSlice(payload["output"]); len(outputItems) > 0 {
		for _, raw := range outputItems {
			itemMap := asMap(raw)
			if itemMap == nil {
				continue
			}
			handleOpenAIOutputItem(itemMap, content, reasoning, state)
		}
	}

	if responseMap := asMap(payload["response"]); responseMap != nil {
		contentText, reasoningText, toolCalls, modelValue := extractLLMResponseFromJSON(responseMap, "openai-compatible")
		if model != nil && *model == "" && modelValue != "" {
			*model = modelValue
		}
		if content.Len() == 0 && contentText != "" {
			content.WriteString(contentText)
		}
		if reasoning.Len() == 0 && reasoningText != "" {
			reasoning.WriteString(reasoningText)
		}
		if len(state.toolCalls) == 0 && toolCalls != nil {
			if calls, ok := toolCalls.([]interface{}); ok {
				state.toolCalls = append(state.toolCalls, calls...)
			}
		}
	}

	return true
}

func handleOpenAIOutputItem(item map[string]interface{}, content *strings.Builder, reasoning *strings.Builder, state *openAIResponseStreamState) {
	if item == nil {
		return
	}
	itemType := strings.ToLower(asStringField(item, "type"))
	itemID := asStringField(item, "id")
	if itemType == "message" {
		contentText, reasoningText, toolCalls := parseOpenAIResponseContentParts(item["content"])
		if contentText != "" && (state == nil || !state.hasSeenText(itemID, nil)) {
			content.WriteString(contentText)
			if state != nil {
				state.markTextSeen(itemID, nil)
			}
		}
		if reasoningText != "" && (state == nil || !state.hasSeenReasoning(itemID, nil)) {
			reasoning.WriteString(reasoningText)
			if state != nil {
				state.markReasoningSeen(itemID, nil)
			}
		}
		if len(toolCalls) > 0 && state != nil {
			appendToolCalls(state, toolCalls)
		}
		return
	}
	if itemType == "reasoning" {
		summary := item["summary"]
		if summary == nil {
			summary = item["content"]
		}
		if text := extractTextFromContentRaw(summary); text != "" && (state == nil || !state.hasSeenReasoning(itemID, nil)) {
			reasoning.WriteString(text)
			if state != nil {
				state.markReasoningSeen(itemID, nil)
			}
		}
		return
	}
	if call := buildToolCallFromOutputItem(item); call != nil {
		if state != nil {
			upsertToolCall(&state.toolCalls, call)
		}
	}
}

func buildContentIndexKey(itemID string, contentIndex interface{}) string {
	if itemID == "" && contentIndex == nil {
		return ""
	}
	index := ""
	switch value := contentIndex.(type) {
	case float64:
		index = strconv.FormatInt(int64(value), 10)
	case int:
		index = strconv.Itoa(value)
	case int64:
		index = strconv.FormatInt(value, 10)
	case string:
		index = value
	}
	if index == "" {
		return itemID
	}
	if itemID == "" {
		return index
	}
	return itemID + ":" + index
}

func appendToolCalls(state *openAIResponseStreamState, calls []interface{}) {
	if state == nil || len(calls) == 0 {
		return
	}
	for _, call := range calls {
		upsertToolCall(&state.toolCalls, call)
	}
}

func upsertToolCall(toolCalls *[]interface{}, call interface{}) {
	if toolCalls == nil {
		return
	}
	callMap := asMap(call)
	if callMap == nil {
		*toolCalls = append(*toolCalls, call)
		return
	}
	callID := extractToolCallID(callMap)
	if callID == "" {
		*toolCalls = append(*toolCalls, callMap)
		return
	}
	for i, existing := range *toolCalls {
		existingMap := asMap(existing)
		if existingMap == nil {
			continue
		}
		if extractToolCallID(existingMap) != callID {
			continue
		}
		merged := make(map[string]interface{}, len(existingMap)+len(callMap))
		for key, value := range existingMap {
			merged[key] = value
		}
		for key, value := range callMap {
			merged[key] = value
		}
		(*toolCalls)[i] = merged
		return
	}
	*toolCalls = append(*toolCalls, callMap)
}

func extractToolCallID(call map[string]interface{}) string {
	if call == nil {
		return ""
	}
	if id := asStringField(call, "id"); id != "" {
		return id
	}
	if id := asStringField(call, "call_id"); id != "" {
		return id
	}
	if id := asStringField(call, "tool_call_id"); id != "" {
		return id
	}
	return ""
}

func parseOpenAIResponseContentParts(contentVal interface{}) (string, string, []interface{}) {
	parts := asSlice(contentVal)
	if len(parts) == 0 {
		if text := extractTextFromContentRaw(contentVal); text != "" {
			return text, "", nil
		}
		return "", "", nil
	}

	var content strings.Builder
	var reasoning strings.Builder
	var toolCalls []interface{}

	for _, raw := range parts {
		part := asMap(raw)
		if part == nil {
			if text := extractTextFromContentRaw(raw); text != "" {
				content.WriteString(text)
			}
			continue
		}
		partType := asStringField(part, "type")
		switch partType {
		case "output_text", "text":
			if text := asStringField(part, "text"); text != "" {
				content.WriteString(text)
				continue
			}
			if text := extractTextFromContentRaw(part["content"]); text != "" {
				content.WriteString(text)
			}
		case "reasoning", "thinking", "analysis":
			if text := asStringField(part, "text"); text != "" {
				reasoning.WriteString(text)
				continue
			}
			if text := asStringField(part, "summary"); text != "" {
				reasoning.WriteString(text)
				continue
			}
			if text := extractTextFromContentRaw(part["content"]); text != "" {
				reasoning.WriteString(text)
			}
		case "tool_call", "function_call":
			if call := buildToolCallFromOutputItem(part); call != nil {
				toolCalls = append(toolCalls, call)
			}
		default:
			if text := extractTextFromContentRaw(part); text != "" {
				content.WriteString(text)
			}
		}
	}

	return content.String(), reasoning.String(), toolCalls
}

func buildToolCallFromOutputItem(item map[string]interface{}) map[string]interface{} {
	if item == nil {
		return nil
	}

	itemType := strings.ToLower(asStringField(item, "type"))
	if itemType == "message" || itemType == "reasoning" || itemType == "output_text" || itemType == "text" {
		return nil
	}

	if function := asMap(item["function"]); function != nil {
		call := map[string]interface{}{
			"type":     "function",
			"function": function,
		}
		if id := asStringField(item, "id"); id != "" {
			call["id"] = id
		} else if id := asStringField(item, "call_id"); id != "" {
			call["id"] = id
		}
		return call
	}

	name := asStringField(item, "name")
	if name == "" {
		name = asStringField(item, "tool_name")
	}
	args := asStringField(item, "arguments")
	if args == "" {
		args = asStringField(item, "args")
	}

	if name == "" && args == "" {
		if isToolLikeType(itemType) || hasToolCallIdentity(item) {
			return item
		}
		return nil
	}

	call := map[string]interface{}{
		"type": "function",
	}
	if id := asStringField(item, "id"); id != "" {
		call["id"] = id
	} else if id := asStringField(item, "call_id"); id != "" {
		call["id"] = id
	}
	function := map[string]interface{}{}
	if name != "" {
		function["name"] = name
	}
	if args != "" {
		function["arguments"] = args
	}
	if len(function) > 0 {
		call["function"] = function
	}

	return call
}

func isToolLikeType(itemType string) bool {
	if itemType == "" {
		return false
	}
	return strings.Contains(itemType, "tool") || strings.HasSuffix(itemType, "_call") || strings.Contains(itemType, "function_call")
}

func hasToolCallIdentity(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	if asStringField(item, "call_id") != "" || asStringField(item, "tool_call_id") != "" {
		return true
	}
	if asStringField(item, "name") != "" || asStringField(item, "tool_name") != "" {
		return true
	}
	if _, ok := item["arguments"]; ok {
		return true
	}
	if _, ok := item["tool"]; ok {
		return true
	}
	return false
}

func parseClaudeSSEPayload(payload map[string]interface{}, content *strings.Builder, reasoning *strings.Builder, toolCalls *[]interface{}) {
	payloadType := asStringField(payload, "type")
	switch payloadType {
	case "content_block_start":
		if block := asMap(payload["content_block"]); block != nil {
			blockType := asStringField(block, "type")
			switch blockType {
			case "tool_use":
				*toolCalls = append(*toolCalls, block)
			case "thinking":
				if text := extractTextFromContentRaw(block["thinking"]); text != "" {
					reasoning.WriteString(text)
				}
			case "text":
				if text := extractTextFromContentRaw(block["text"]); text != "" {
					content.WriteString(text)
				}
			}
		}
	case "content_block_delta":
		if delta := asMap(payload["delta"]); delta != nil {
			if text := extractTextFromContentRaw(delta["text"]); text != "" {
				content.WriteString(text)
			}
			if reasoningText := extractTextFromContentRaw(delta["thinking"]); reasoningText != "" {
				reasoning.WriteString(reasoningText)
			}
			if partial := extractTextFromContentRaw(delta["partial_json"]); partial != "" {
				appendToolCallInput(toolCalls, partial)
			}
		}
	}
}

func parseGeminiSSEPayload(payload map[string]interface{}, content *strings.Builder, reasoning *strings.Builder, toolCalls *[]interface{}) {
	candidates := asSlice(payload["candidates"])
	if len(candidates) == 0 {
		return
	}
	candidate := asMap(candidates[0])
	if candidate == nil {
		return
	}
	contentMap := asMap(candidate["content"])
	if contentMap == nil {
		return
	}
	parts := asSlice(contentMap["parts"])
	partText, partReasoning, partToolCalls := parseGeminiParts(parts)
	if partText != "" {
		content.WriteString(partText)
	}
	if partReasoning != "" {
		reasoning.WriteString(partReasoning)
	}
	if len(partToolCalls) > 0 {
		*toolCalls = append(*toolCalls, partToolCalls...)
	}
}

func parseOllamaSSEPayload(payload map[string]interface{}, content *strings.Builder, toolCalls *[]interface{}) {
	if text := asStringField(payload, "response"); text != "" {
		content.WriteString(text)
	}
	if message := asMap(payload["message"]); message != nil {
		if text := extractTextFromContentRaw(message["content"]); text != "" {
			content.WriteString(text)
		}
		if calls := normalizeToolCalls(message["tool_calls"]); len(calls) > 0 {
			*toolCalls = append(*toolCalls, calls...)
		}
	}
	if calls := normalizeToolCalls(payload["tool_calls"]); len(calls) > 0 {
		*toolCalls = append(*toolCalls, calls...)
	}
}

func extractToolCallsFromRequest(payload map[string]interface{}) interface{} {
	if calls := normalizeToolCalls(payload["tool_calls"]); len(calls) > 0 {
		return calls
	}
	if calls := normalizeToolCalls(payload["toolCalls"]); len(calls) > 0 {
		return calls
	}
	if messages := asSlice(payload["messages"]); len(messages) > 0 {
		if calls := extractToolCallsFromMessages(messages); len(calls) > 0 {
			return calls
		}
	}
	if contents := asSlice(payload["contents"]); len(contents) > 0 {
		if calls := extractToolCallsFromContents(contents); len(calls) > 0 {
			return calls
		}
	}
	return nil
}

func buildPromptFromMessages(messages []interface{}) string {
	return buildPromptFromRoleContent(messages, func(item map[string]interface{}) (string, string) {
		role := asStringField(item, "role")
		content := extractTextFromContent(item["content"])
		if content == "" {
			content = extractTextFromContent(item["parts"])
		}
		return role, content
	})
}

func buildPromptFromContents(contents []interface{}) string {
	return buildPromptFromRoleContent(contents, func(item map[string]interface{}) (string, string) {
		role := asStringField(item, "role")
		content := extractTextFromContent(item["parts"])
		if content == "" {
			content = extractTextFromContent(item["content"])
		}
		return role, content
	})
}

func buildPromptFromInput(input interface{}) string {
	switch value := input.(type) {
	case string:
		return strings.TrimSpace(value)
	case []interface{}:
		return buildPromptFromMessages(value)
	case map[string]interface{}:
		return extractTextFromContent(value)
	default:
		return ""
	}
}

func buildPromptFromPrompt(prompt interface{}) string {
	switch value := prompt.(type) {
	case string:
		return strings.TrimSpace(value)
	case []interface{}:
		var parts []string
		for _, item := range value {
			if text := extractTextFromContent(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	default:
		return ""
	}
}

func mergePromptWithSystem(prompt string, system string) string {
	system = strings.TrimSpace(system)
	if system == "" {
		return prompt
	}
	block := buildPromptBlock("system", system)
	if prompt == "" {
		return block
	}
	return strings.Join([]string{block, prompt}, "\n\n")
}

func buildPromptFromRoleContent(items []interface{}, extractor func(map[string]interface{}) (string, string)) string {
	var parts []string
	for _, raw := range items {
		item := asMap(raw)
		if item == nil {
			continue
		}
		role, content := extractor(item)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		parts = append(parts, buildPromptBlock(role, content))
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func buildPromptBlock(role string, content string) string {
	if content == "" {
		return ""
	}
	if role == "" {
		return content
	}
	return "**" + role + "**:\n" + content
}

func extractTextFromContentRaw(content interface{}) string {
	return extractTextFromContentWithTrim(content, false)
}

func extractTextFromContent(content interface{}) string {
	return extractTextFromContentWithTrim(content, true)
}

func extractTextFromContentWithTrim(content interface{}, trim bool) string {
	switch value := content.(type) {
	case string:
		if trim {
			return strings.TrimSpace(value)
		}
		return value
	case []interface{}:
		var parts []string
		for _, item := range value {
			if text := extractTextFromContentWithTrim(item, trim); text != "" {
				parts = append(parts, text)
			}
		}
		joined := strings.Join(parts, "\n")
		if trim {
			return strings.TrimSpace(joined)
		}
		return joined
	case map[string]interface{}:
		if text, ok := value["text"].(string); ok {
			if trim {
				return strings.TrimSpace(text)
			}
			return text
		}
		if text, ok := value["input_text"].(string); ok {
			if trim {
				return strings.TrimSpace(text)
			}
			return text
		}
		if text, ok := value["output_text"].(string); ok {
			if trim {
				return strings.TrimSpace(text)
			}
			return text
		}
		if text, ok := value["content"].(string); ok {
			if trim {
				return strings.TrimSpace(text)
			}
			return text
		}
		if contentVal, ok := value["content"]; ok {
			return extractTextFromContentWithTrim(contentVal, trim)
		}
		if partsVal, ok := value["parts"]; ok {
			return extractTextFromContentWithTrim(partsVal, trim)
		}
	}
	return ""
}

func extractToolCallsFromMessages(messages []interface{}) []interface{} {
	var calls []interface{}
	for _, raw := range messages {
		message := asMap(raw)
		if message == nil {
			continue
		}
		if toolCalls := normalizeToolCalls(message["tool_calls"]); len(toolCalls) > 0 {
			calls = append(calls, toolCalls...)
		}
		if functionCall := asMap(message["function_call"]); functionCall != nil {
			calls = append(calls, wrapFunctionCall(functionCall))
		}
		if contentBlocks := asSlice(message["content"]); len(contentBlocks) > 0 {
			for _, block := range contentBlocks {
				blockMap := asMap(block)
				if blockMap == nil {
					continue
				}
				if asStringField(blockMap, "type") == "tool_use" {
					calls = append(calls, blockMap)
				}
			}
		}
	}
	return calls
}

func extractToolCallsFromContents(contents []interface{}) []interface{} {
	var calls []interface{}
	for _, raw := range contents {
		content := asMap(raw)
		if content == nil {
			continue
		}
		parts := asSlice(content["parts"])
		partCalls := extractGeminiToolCalls(parts)
		if len(partCalls) > 0 {
			calls = append(calls, partCalls...)
		}
	}
	return calls
}

func normalizeToolCalls(raw interface{}) []interface{} {
	switch value := raw.(type) {
	case []interface{}:
		return value
	case map[string]interface{}:
		return []interface{}{value}
	default:
		return nil
	}
}

func wrapFunctionCall(call map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"type":     "function",
		"function": call,
	}
}

func parseClaudeContentBlocks(blocks []interface{}) (string, string, []interface{}) {
	var content strings.Builder
	var reasoning strings.Builder
	var toolCalls []interface{}

	for _, raw := range blocks {
		block := asMap(raw)
		if block == nil {
			continue
		}
		blockType := asStringField(block, "type")
		switch blockType {
		case "text":
			if text := extractTextFromContent(block["text"]); text != "" {
				content.WriteString(text)
			}
		case "thinking":
			if text := extractTextFromContent(block["thinking"]); text != "" {
				reasoning.WriteString(text)
			}
		case "tool_use":
			toolCalls = append(toolCalls, block)
		}
	}

	return strings.TrimSpace(content.String()), strings.TrimSpace(reasoning.String()), toolCalls
}

func parseGeminiParts(parts []interface{}) (string, string, []interface{}) {
	var content strings.Builder
	var reasoning strings.Builder
	var toolCalls []interface{}

	for _, raw := range parts {
		part := asMap(raw)
		if part == nil {
			continue
		}
		if text := extractTextFromContent(part["text"]); text != "" {
			content.WriteString(text)
		}
		if thought := extractTextFromContent(part["thought"]); thought != "" {
			reasoning.WriteString(thought)
		}
		if fc := asMap(part["functionCall"]); fc != nil {
			toolCalls = append(toolCalls, wrapFunctionCall(fc))
		}
	}

	return strings.TrimSpace(content.String()), strings.TrimSpace(reasoning.String()), toolCalls
}

func extractGeminiToolCalls(parts []interface{}) []interface{} {
	var toolCalls []interface{}
	for _, raw := range parts {
		part := asMap(raw)
		if part == nil {
			continue
		}
		if fc := asMap(part["functionCall"]); fc != nil {
			toolCalls = append(toolCalls, wrapFunctionCall(fc))
		}
	}
	return toolCalls
}

func extractReasoningFromMap(value map[string]interface{}) string {
	if value == nil {
		return ""
	}
	if text := asStringField(value, "reasoning"); text != "" {
		return text
	}
	if text := asStringField(value, "reasoning_content"); text != "" {
		return text
	}
	if text := asStringField(value, "thinking"); text != "" {
		return text
	}
	if text := asStringField(value, "thoughts"); text != "" {
		return text
	}
	return ""
}

func mergeOpenAIToolCalls(existing []map[string]interface{}, deltas []interface{}) []map[string]interface{} {
	for _, raw := range deltas {
		delta := asMap(raw)
		if delta == nil {
			continue
		}
		index := int(asFloatField(delta, "index"))
		if index < 0 {
			index = 0
		}
		for len(existing) <= index {
			existing = append(existing, map[string]interface{}{})
		}
		target := existing[index]
		if id := asStringField(delta, "id"); id != "" {
			target["id"] = id
		}
		if callType := asStringField(delta, "type"); callType != "" {
			target["type"] = callType
		}
		if fn := asMap(delta["function"]); fn != nil {
			fnTarget := asMap(target["function"])
			if fnTarget == nil {
				fnTarget = map[string]interface{}{}
			}
			if name := asStringField(fn, "name"); name != "" {
				fnTarget["name"] = name
			}
			if args := asStringField(fn, "arguments"); args != "" {
				if existingArgs := asString(fnTarget["arguments"]); existingArgs != "" {
					fnTarget["arguments"] = existingArgs + args
				} else {
					fnTarget["arguments"] = args
				}
			}
			target["function"] = fnTarget
		}
		existing[index] = target
	}
	return existing
}

func appendToolCallInput(toolCalls *[]interface{}, chunk string) {
	if toolCalls == nil || len(*toolCalls) == 0 {
		return
	}
	last := (*toolCalls)[len(*toolCalls)-1]
	lastMap := asMap(last)
	if lastMap == nil {
		return
	}
	input, ok := lastMap["input"].(string)
	if ok {
		lastMap["input"] = input + chunk
		return
	}
	lastMap["input"] = chunk
	(*toolCalls)[len(*toolCalls)-1] = lastMap
}

func asSlice(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if slice, ok := value.([]interface{}); ok {
		return slice
	}
	return nil
}

func asMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	if m, ok := value.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func asStringField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	return asString(m[key])
}

func asString(value interface{}) string {
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func asFloatField(m map[string]interface{}, key string) float64 {
	if m == nil {
		return 0
	}
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func asBool(m map[string]interface{}, key string) (bool, bool) {
	if m == nil {
		return false, false
	}
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b, true
		}
	}
	return false, false
}
