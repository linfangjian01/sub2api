package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func isBareAntigravityCarbonModel(model string) bool {
	model = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(model)), "models/")
	return model == "gemini-4-carbon" || model == "gemini-4-hydrogen"
}

func carbonThinkingLevelFromClaudeBody(body []byte) string {
	return gemini4ThinkingLevelFromClaudeBody("gemini-4-carbon", body)
}

func gemini4ThinkingLevelFromClaudeBody(model string, body []byte) string {
	normalized := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(model)), "models/")
	if normalized == "gemini-4-hydrogen" {
		if strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "thinking.type").String())) != "disabled" {
			switch strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "output_config.effort").String())) {
			case "xhigh", "max":
				return "xhigh"
			}
		}
	}
	level := geminiThinkingLevelFor(normalized, parseClaudeThinkingIntent(body))
	if level == "" {
		return "high"
	}
	return level
}

// The selected Antigravity Carbon/Hydrogen model ID already determines the thinking tier.
func removeCarbonTierThinkingSelector(body []byte, mappedModel string) ([]byte, error) {
	model := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(mappedModel)), "models/")
	switch model {
	case "gemini-4-carbon-low", "gemini-4-carbon-medium", "gemini-4-carbon-high",
		"gemini-4-hydrogen-low", "gemini-4-hydrogen-medium", "gemini-4-hydrogen-high", "gemini-4-hydrogen-xhigh":
	default:
		return body, nil
	}

	var err error
	for _, generationKey := range []string{"generationConfig", "generation_config"} {
		for _, thinkingKey := range []string{"thinkingConfig", "thinking_config"} {
			configurationPath := generationKey + "." + thinkingKey
			for _, selector := range []string{"thinkingLevel", "thinking_level", "thinkingBudget", "thinking_budget"} {
				body, err = sjson.DeleteBytes(body, configurationPath+"."+selector)
				if err != nil {
					return nil, err
				}
			}
			configuration := gjson.GetBytes(body, configurationPath)
			if configuration.Exists() && configuration.IsObject() && len(configuration.Map()) == 0 {
				body, err = sjson.DeleteBytes(body, configurationPath)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return body, nil
}
