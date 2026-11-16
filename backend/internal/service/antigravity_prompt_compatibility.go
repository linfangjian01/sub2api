package service

import (
	"fmt"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type antigravityPromptProtocol uint8

const (
	antigravityPromptClaude antigravityPromptProtocol = iota
	antigravityPromptGemini
	antigravityPromptResponses
	antigravityPromptChatCompletions
)

const antigravityCodingAssistantIdentity = "You are a helpful coding assistant."

var antigravityPromptReplacements = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)\bYou are Codex,\s*(?:a coding agent(?: based on GPT-[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)?|(?:an agent )?based on GPT-[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*)\.`),
		replacement: antigravityCodingAssistantIdentity,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bYou are Claude Code, Anthropic['\x{2019}]s official CLI for Claude\.`),
		replacement: antigravityCodingAssistantIdentity,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bYou are a Claude agent, built on Anthropic['\x{2019}]s Claude Agent SDK\.`),
		replacement: antigravityCodingAssistantIdentity,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bYou are an AI coding assistant, powered by gemini-[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.`),
		replacement: antigravityCodingAssistantIdentity,
	},
	{
		pattern:     regexp.MustCompile(`(?i)\bYou are a coding agent in the Cursor IDE that helps the USER with software engineering tasks\.`),
		replacement: antigravityCodingAssistantIdentity,
	},
	{
		pattern:     regexp.MustCompile(`(?im)^[ \t]*x-anthropic-billing-header:[ \t]*(?:[A-Za-z_][A-Za-z0-9_]*=[A-Za-z0-9_.:/-]+;[ \t]*)+\r?$`),
		replacement: "",
	},
}

func replaceAntigravitySystemPrompt(text string) string {
	for _, rule := range antigravityPromptReplacements {
		text = rule.pattern.ReplaceAllString(text, rule.replacement)
	}
	return text
}

// Select fields before replacing text, so user content, paths and tool payloads
// elsewhere in the JSON body are never rewritten by a whole-request regexp.
func rewriteAntigravitySystemPrompts(c *gin.Context, body []byte, protocol antigravityPromptProtocol) ([]byte, error) {
	if c == nil {
		return body, nil
	}
	group := apiKeyGroup(getAPIKeyFromContext(c))
	if group == nil || group.Platform != PlatformAntigravity || !gjson.ValidBytes(body) {
		return body, nil
	}

	original := gjson.ParseBytes(body)
	rewrittenBody := body
	rewriteText := func(path string, value gjson.Result) error {
		if value.Type != gjson.String {
			return nil
		}
		text := value.String()
		rewrittenText := replaceAntigravitySystemPrompt(text)
		if rewrittenText == text {
			return nil
		}
		var err error
		rewrittenBody, err = sjson.SetBytes(rewrittenBody, path, rewrittenText)
		return err
	}
	rewriteContent := func(path string, content gjson.Result) error {
		if content.Type == gjson.String {
			return rewriteText(path, content)
		}
		if !content.IsArray() {
			return nil
		}
		for partIndex, part := range content.Array() {
			switch part.Get("type").String() {
			case "text", "input_text", "output_text":
				if err := rewriteText(fmt.Sprintf("%s.%d.text", path, partIndex), part.Get("text")); err != nil {
					return err
				}
			}
		}
		return nil
	}

	switch protocol {
	case antigravityPromptClaude:
		if err := rewriteContent("system", original.Get("system")); err != nil {
			return nil, err
		}
	case antigravityPromptGemini:
		for _, instructionField := range []string{"systemInstruction", "system_instruction"} {
			for partIndex, part := range original.Get(instructionField + ".parts").Array() {
				path := fmt.Sprintf("%s.parts.%d.text", instructionField, partIndex)
				if err := rewriteText(path, part.Get("text")); err != nil {
					return nil, err
				}
			}
		}
	case antigravityPromptResponses, antigravityPromptChatCompletions:
		messageField := "messages"
		if protocol == antigravityPromptResponses {
			messageField = "input"
			if err := rewriteText("instructions", original.Get("instructions")); err != nil {
				return nil, err
			}
		}
		for messageIndex, message := range original.Get(messageField).Array() {
			role := message.Get("role").String()
			if role != "system" && role != "developer" {
				continue
			}
			path := fmt.Sprintf("%s.%d.content", messageField, messageIndex)
			if err := rewriteContent(path, message.Get("content")); err != nil {
				return nil, err
			}
		}
	}
	return rewrittenBody, nil
}
