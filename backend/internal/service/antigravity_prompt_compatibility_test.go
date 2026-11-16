package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAntigravityPromptCompatibilityRules(t *testing.T) {
	for _, testCase := range []struct {
		name string
		text string
		want string
	}{
		{"codex agent", "You are Codex, a coding agent.", antigravityCodingAssistantIdentity},
		{"codex model", "You are Codex, based on GPT-6.1-sol.", antigravityCodingAssistantIdentity},
		{"codex model agent", "You are Codex, an agent based on GPT-6.1-SOL.", antigravityCodingAssistantIdentity},
		{"codex existing variant", "You are Codex, a coding agent based on GPT-6.1.", antigravityCodingAssistantIdentity},
		{"case insensitive", "YOU ARE CODEX,\n based on gpt-6.1.", antigravityCodingAssistantIdentity},
		{"claude cli", "You are Claude Code, Anthropic's official CLI for Claude.", antigravityCodingAssistantIdentity},
		{"claude cli curly apostrophe", "You are Claude Code, Anthropic\u2019s official CLI for Claude.", antigravityCodingAssistantIdentity},
		{"claude sdk", "You are a Claude agent, built on Anthropic's Claude Agent SDK.", antigravityCodingAssistantIdentity},
		{"claude sdk curly apostrophe", "You are a Claude agent, built on Anthropic\u2019s Claude Agent SDK.", antigravityCodingAssistantIdentity},
		{"cursor sampled model identity", "You are an AI coding assistant, powered by gemini-4-hydrogen-xhigh.", antigravityCodingAssistantIdentity},
		{"cursor sampled IDE identity", "You are a coding agent in the Cursor IDE that helps the USER with software engineering tasks.", antigravityCodingAssistantIdentity},
		{"cursor model variant", "You are an AI coding assistant, powered by gemini-3.1-pro-high.", antigravityCodingAssistantIdentity},
		{"cursor ordinary mention", "Use Cursor IDE and gemini-4-hydrogen-xhigh; keep ~/.cursor/rules intact.", "Use Cursor IDE and gemini-4-hydrogen-xhigh; keep ~/.cursor/rules intact."},
		{"embedded identity", "Instructions:\nYou are Codex, a coding agent.\nContinue.", "Instructions:\n" + antigravityCodingAssistantIdentity + "\nContinue."},
		{"multiple identities", "You are Codex, a coding agent. You are Codex, based on GPT-6.", antigravityCodingAssistantIdentity + " " + antigravityCodingAssistantIdentity},
		{"billing only", "x-anthropic-billing-header: cc_version=2.1.271; cc_entrypoint=claude-desktop-3p;", ""},
		{"billing crlf", "Keep.\r\n\tX-Anthropic-Billing-Header: cc_version=2.1; account_id=abc:/x-y;\r\nNext.", "Keep.\r\n\nNext."},
		{"billing malformed", "x-anthropic-billing-header: cc_version=2.1", "x-anthropic-billing-header: cc_version=2.1"},
		{"billing quoted", "Example: x-anthropic-billing-header: cc_version=2.1;", "Example: x-anthropic-billing-header: cc_version=2.1;"},
		{"ordinary vendor mentions", "Use Codex, Claude Code and GPT-6.1 model IDs.", "Use Codex, Claude Code and GPT-6.1 model IDs."},
		{"word boundary", "NotYou are Codex, a coding agent.", "NotYou are Codex, a coding agent."},
		{"opencode excluded", "opencode.session_move opencode /tmp/opencode/file C:\\opencode\\config.json", "opencode.session_move opencode /tmp/opencode/file C:\\opencode\\config.json"},
		{"paths", "/home/claude/CLAUDE.md /tmp/codex/file C:\\Claude Code\\settings.json ~/.cursor/rules", "/home/claude/CLAUDE.md /tmp/codex/file C:\\Claude Code\\settings.json ~/.cursor/rules"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			rewritten := replaceAntigravitySystemPrompt(testCase.text)
			require.Equal(t, testCase.want, rewritten)
			require.Equal(t, rewritten, replaceAntigravitySystemPrompt(rewritten), "replacement must be idempotent")
		})
	}
}

func TestAntigravityPromptCompatibilityFieldAndGroupIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const identity = "You are Codex, a coding agent."
	textBlock := func(blockType string) map[string]any {
		return map[string]any{"type": blockType, "text": identity, "cache_control": map[string]string{"type": "ephemeral"}}
	}
	for _, fixture := range []struct {
		name          string
		protocol      antigravityPromptProtocol
		body          map[string]any
		rewrittenPath []string
		protectedPath []string
	}{
		{"claude string", antigravityPromptClaude, map[string]any{
			"system": identity, "messages": []any{map[string]any{"role": "user", "content": identity}},
		}, []string{"system"}, []string{"messages"}},
		{"claude blocks", antigravityPromptClaude, map[string]any{
			"system":   []any{textBlock("text"), textBlock("thinking")},
			"messages": []any{map[string]any{"role": "user", "content": identity}},
		}, []string{"system.0.text"}, []string{"system.0.cache_control", "system.1", "messages"}},
		{"gemini camel and snake case", antigravityPromptGemini, map[string]any{
			"systemInstruction":  map[string]any{"parts": []any{map[string]any{"text": identity}, map[string]any{"functionCall": map[string]string{"name": identity}}}},
			"system_instruction": map[string]any{"parts": []any{map[string]string{"text": identity}}},
			"contents":           []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": identity}}}},
		}, []string{"systemInstruction.parts.0.text", "system_instruction.parts.0.text"}, []string{"systemInstruction.parts.1", "contents"}},
		{"responses", antigravityPromptResponses, map[string]any{
			"instructions": identity,
			"input": []any{
				map[string]any{"role": "system", "content": identity},
				map[string]any{"role": "developer", "content": []any{textBlock("input_text")}},
				map[string]any{"role": "user", "content": identity},
				map[string]any{"type": "function_call_output", "output": identity},
				map[string]any{"role": "assistant", "content": identity},
			},
		}, []string{"instructions", "input.0.content", "input.1.content.0.text"}, []string{"input.1.content.0.cache_control", "input.2", "input.3", "input.4"}},
		{"chat completions", antigravityPromptChatCompletions, map[string]any{
			"messages": []any{
				map[string]any{"role": "system", "content": []any{textBlock("text")}},
				map[string]any{"role": "developer", "content": identity},
				map[string]any{"role": "user", "content": identity},
				map[string]any{"role": "tool", "content": identity},
				map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"function": map[string]string{"arguments": identity}}}},
			},
		}, []string{"messages.0.content.0.text", "messages.1.content"}, []string{"messages.0.content.0.cache_control", "messages.2", "messages.3", "messages.4"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			fixture.body["tools"] = []any{map[string]any{"name": "opencode.session_move", "description": identity, "parameters": map[string]string{"example": identity}}}
			fixture.body["metadata"] = map[string]string{"identity": identity}
			fixture.body["model"] = "gemini-4-hydrogen-xhigh"
			body, err := json.Marshal(fixture.body)
			require.NoError(t, err)
			originalBytes := append([]byte(nil), body...)
			for _, platform := range []string{PlatformAntigravity, PlatformGemini, PlatformComposite, PlatformOpenAI, PlatformAnthropic} {
				requestContext, _ := newAntigravityCompatContext(http.MethodPost, "/test", body)
				requestContext.Set("api_key", &APIKey{Group: &Group{Platform: platform}})
				rewritten, err := rewriteAntigravitySystemPrompts(requestContext, body, fixture.protocol)
				require.NoError(t, err)
				require.Equal(t, originalBytes, body, "do not mutate caller bytes")
				if platform != PlatformAntigravity {
					require.Equal(t, body, rewritten, platform)
					continue
				}
				for _, path := range fixture.rewrittenPath {
					require.Equal(t, antigravityCodingAssistantIdentity, gjson.GetBytes(rewritten, path).String(), path)
				}
				for _, path := range append(fixture.protectedPath, "tools", "metadata", "model") {
					require.Equal(t, gjson.GetBytes(body, path).Raw, gjson.GetBytes(rewritten, path).Raw, path)
				}
				again, err := rewriteAntigravitySystemPrompts(requestContext, rewritten, fixture.protocol)
				require.NoError(t, err)
				require.Equal(t, rewritten, again)
			}
		})
	}

	for _, body := range [][]byte{nil, []byte(`{"system":`), []byte(`{"system":"unchanged"}`)} {
		requestContext, _ := newAntigravityCompatContext(http.MethodPost, "/test", body)
		for _, apiKey := range []*APIKey{nil, {}, {Group: &Group{Platform: PlatformAntigravity}}} {
			requestContext.Set("api_key", apiKey)
			rewritten, err := rewriteAntigravitySystemPrompts(requestContext, body, antigravityPromptClaude)
			require.NoError(t, err)
			require.Equal(t, body, rewritten)
		}
	}
}

func TestAntigravityPromptCompatibilityForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const identity = "You are Codex, a coding agent."
	const prompt = "x-anthropic-billing-header: cc_version=2.1;\nYou are Codex, a coding agent.\nYou are Claude Code, Anthropic's official CLI for Claude.\nYou are a Claude agent, built on Anthropic's Claude Agent SDK.\nKeep /tmp/opencode/project and C:\\Codex\\file intact."
	const cursorPrompt = "You are an AI coding assistant, powered by gemini-4-hydrogen-xhigh.\nYou are a coding agent in the Cursor IDE that helps the USER with software engineering tasks."
	for _, protocol := range []string{"responses", "chat", "messages", "gemini"} {
		for _, model := range []string{"gemini-3.1-pro-high", "claude-sonnet-4-6"} {
			if protocol == "gemini" && strings.HasPrefix(model, "claude-") {
				continue
			}
			t.Run(fmt.Sprintf("%s/%s", protocol, model), func(t *testing.T) {
				request := map[string]any{"model": model, "max_tokens": 128}
				switch protocol {
				case "responses":
					request["instructions"] = prompt + "\n" + cursorPrompt
					request["input"] = []any{map[string]any{"role": "developer", "content": identity}, map[string]any{"role": "user", "content": identity}}
				case "chat":
					request["messages"] = []any{map[string]any{"role": "system", "content": prompt + "\n" + cursorPrompt}, map[string]any{"role": "developer", "content": identity}, map[string]any{"role": "user", "content": identity}}
				case "messages":
					request["system"] = []any{map[string]string{"type": "text", "text": prompt + "\n" + cursorPrompt}}
					request["messages"] = []any{map[string]any{"role": "user", "content": identity}}
				case "gemini":
					request["systemInstruction"] = map[string]any{"parts": []any{map[string]string{"text": prompt + "\n" + cursorPrompt}}}
					request["contents"] = []any{map[string]any{"role": "user", "parts": []any{map[string]string{"text": identity}}}}
				}
				body, err := json.Marshal(request)
				require.NoError(t, err)
				var forwarded []byte
				upstream := &queuedHTTPUpstreamStub{
					responses: []*http.Response{antigravityCompatSuccessResponse()},
					onCall: func(upstreamRequest *http.Request, _ *queuedHTTPUpstreamStub) {
						forwarded, err = io.ReadAll(upstreamRequest.Body)
						require.NoError(t, err)
					},
				}
				gateway := newAntigravityCompatService(config.GatewayConfig{MaxLineSize: defaultMaxLineSize}, upstream)
				account := newAntigravityCompatAccount(AccountTypeOAuth)
				requestContext, _ := newAntigravityCompatContext(http.MethodPost, "/test", body)
				requestContext.Set("api_key", &APIKey{Group: &Group{ID: 8, Platform: PlatformAntigravity}})
				switch protocol {
				case "responses":
					_, err = gateway.ForwardAsResponses(context.Background(), requestContext, account, body, nil)
				case "chat":
					_, err = gateway.ForwardAsChatCompletions(context.Background(), requestContext, account, body, nil)
				case "messages":
					_, err = gateway.Forward(context.Background(), requestContext, account, body, false)
				case "gemini":
					_, err = gateway.ForwardGemini(context.Background(), requestContext, account, model, "generateContent", false, body, false)
				}
				require.NoError(t, err)
				systemText := gjson.GetBytes(forwarded, "request.systemInstruction.parts").Raw
				require.Contains(t, systemText, antigravityCodingAssistantIdentity)
				require.NotContains(t, systemText, "You are Codex")
				require.NotContains(t, systemText, "You are Claude Code")
				require.NotContains(t, systemText, "You are a Claude agent")
				require.NotContains(t, systemText, "powered by gemini-")
				require.NotContains(t, systemText, "in the Cursor IDE")
				require.NotContains(t, systemText, "x-anthropic-billing-header")
				require.Contains(t, systemText, "/tmp/opencode/project")
				require.Contains(t, systemText, "You are Antigravity")
				require.Equal(t, identity, gjson.GetBytes(forwarded, "request.contents.0.parts.0.text").String())
				require.Equal(t, model, gjson.GetBytes(forwarded, "model").String())
			})
		}
	}
}
