package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// HarnessName identifies a tool-call harness.
type HarnessName string

const (
	HarnessNative HarnessName = "native"
	HarnessHermes HarnessName = "hermes"
	HarnessQwen   HarnessName = "qwen"
)

// ToolCallParts is a parsed tool invocation emitted by a tagged harness.
type ToolCallParts struct {
	ID        string
	Name      string
	Arguments string
}

// Harness adapts the system prompt and parses tool calls for different model conventions.
type Harness interface {
	Name() HarnessName
	Prepare(systemPrompt string, tools []ToolDefinition) (preparedSystemPrompt string, requestTools []ToolDefinition)
	Parse(content string) (parsedContent string, toolCalls []ToolCallParts)
}

// ResolveHarness returns the named harness, defaulting to native.
func ResolveHarness(name string) Harness {
	switch HarnessName(name) {
	case HarnessHermes:
		return taggedHarness(HarnessHermes, "tool_call")
	case HarnessQwen:
		return taggedHarness(HarnessQwen, "function_call")
	default:
		return nativeHarness{}
	}
}

type nativeHarness struct{}

func (nativeHarness) Name() HarnessName { return HarnessNative }

func (nativeHarness) Prepare(systemPrompt string, tools []ToolDefinition) (string, []ToolDefinition) {
	return systemPrompt, tools
}

func (nativeHarness) Parse(content string) (string, []ToolCallParts) {
	return content, nil
}

func taggedHarness(name HarnessName, tag string) Harness {
	open := regexp.QuoteMeta("<" + tag + ">")
	close := regexp.QuoteMeta("</" + tag + ">")
	block := regexp.MustCompile(open + `([\s\S]*?)` + close + `\s*`)
	return taggedHarnessImpl{
		name:  name,
		tag:   tag,
		block: block,
	}
}

type taggedHarnessImpl struct {
	name  HarnessName
	tag   string
	block *regexp.Regexp
}

func (h taggedHarnessImpl) Name() HarnessName { return h.name }

func (h taggedHarnessImpl) Prepare(systemPrompt string, tools []ToolDefinition) (string, []ToolDefinition) {
	var toolLines []string
	for _, t := range tools {
		b, _ := json.Marshal(t)
		toolLines = append(toolLines, string(b))
	}
	prepared := strings.Join([]string{
		systemPrompt,
		"",
		"You have access to the following tools. Tool schemas are provided inside <tools></tools>:",
		"<tools>",
		strings.Join(toolLines, "\n"),
		"</tools>",
		"",
		fmt.Sprintf("To call a tool, emit exactly one JSON object per call wrapped in <%s></%s> tags, like:", h.tag, h.tag),
		fmt.Sprintf("<%s>{\"name\": \"tool-name\", \"arguments\": {\"arg\": \"value\"}}</%s>", h.tag, h.tag),
	}, "\n")
	return prepared, nil
}

func (h taggedHarnessImpl) Parse(content string) (string, []ToolCallParts) {
	var toolCalls []ToolCallParts
	idx := 0
	cleaned := h.block.ReplaceAllStringFunc(content, func(match string) string {
		body := h.block.FindStringSubmatch(match)[1]
		call := parseTaggedCall(body, idx)
		if call == nil {
			return match
		}
		toolCalls = append(toolCalls, *call)
		idx++
		return ""
	})
	return strings.TrimSpace(cleaned), toolCalls
}

func parseTaggedCall(body string, index int) *ToolCallParts {
	var parsed struct {
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &parsed); err != nil {
		return nil
	}
	if parsed.Name == "" {
		return nil
	}
	var args string
	switch v := parsed.Arguments.(type) {
	case string:
		args = v
	case nil:
		args = "{}"
	default:
		b, _ := json.Marshal(v)
		args = string(b)
	}
	return &ToolCallParts{
		ID:        fmt.Sprintf("call_%d", index),
		Name:      parsed.Name,
		Arguments: args,
	}
}
