package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCarbonRoutesBareModelToAdvertisedTier(t *testing.T) {
	account := &Account{
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"},
		},
	}
	service := &AntigravityGatewayService{}
	for _, testCase := range []struct {
		level string
		want  string
	}{
		{level: "", want: "gemini-4-carbon-high"},
		{level: "low", want: "gemini-4-carbon-low"},
		{level: "medium", want: "gemini-4-carbon-medium"},
		{level: "high", want: "gemini-4-carbon-high"},
	} {
		t.Run(testCase.level, func(t *testing.T) {
			require.Equal(t, testCase.want, service.getMappedModelForThinkingLevel(account, "gemini-4-carbon", testCase.level))
		})
	}
}

func TestCarbonClaudeEffortSelectsTier(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
		want string
	}{
		{name: "default", body: `{}`, want: "high"},
		{name: "low effort", body: `{"output_config":{"effort":"low"}}`, want: "low"},
		{name: "medium effort", body: `{"output_config":{"effort":"medium"}}`, want: "medium"},
		{name: "high effort", body: `{"output_config":{"effort":"high"}}`, want: "high"},
		{name: "disabled takes precedence", body: `{"thinking":{"type":"disabled"},"output_config":{"effort":"high"}}`, want: "low"},
		{name: "medium budget", body: `{"thinking":{"type":"enabled","budget_tokens":4096}}`, want: "medium"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, carbonThinkingLevelFromClaudeBody([]byte(testCase.body)))
		})
	}
}

func TestCarbonTierRemovesNativeThinkingSelector(t *testing.T) {
	body := []byte(`{"contents":[],"generationConfig":{"maxOutputTokens":512,"thinkingConfig":{"thinkingLevel":"low","thinkingBudget":1024,"includeThoughts":true}}}`)
	result, err := removeCarbonTierThinkingSelector(body, "gemini-4-carbon-low")
	require.NoError(t, err)
	require.JSONEq(t, `{"contents":[],"generationConfig":{"maxOutputTokens":512,"thinkingConfig":{"includeThoughts":true}}}`, string(result))

	unchanged, err := removeCarbonTierThinkingSelector(body, "gemini-3.8-flash-low")
	require.NoError(t, err)
	require.Equal(t, body, unchanged)
}

func TestCarbonTierPreservesExplicitAccountOverride(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		mapping map[string]any
		want    string
	}{
		{name: "exact", mapping: map[string]any{"gemini-4-carbon": "custom-model"}, want: "custom-model"},
		{name: "wildcard", mapping: map[string]any{"gemini-*": "custom-model"}, want: "custom-model"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			account := &Account{Platform: PlatformAntigravity, Credentials: map[string]any{"model_mapping": testCase.mapping}}
			service := &AntigravityGatewayService{}
			require.Equal(t, testCase.want, service.getMappedModelForThinkingLevel(account, "gemini-4-carbon", "low"))
		})
	}
}

func TestHydrogenRoutesBareModelToAdvertisedTier(t *testing.T) {
	account := &Account{
		Platform: PlatformAntigravity,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"},
		},
	}
	service := &AntigravityGatewayService{}
	for _, testCase := range []struct {
		level string
		want  string
	}{
		{level: "", want: "gemini-4-hydrogen-high"},
		{level: "low", want: "gemini-4-hydrogen-low"},
		{level: "medium", want: "gemini-4-hydrogen-medium"},
		{level: "high", want: "gemini-4-hydrogen-high"},
		{level: "xhigh", want: "gemini-4-hydrogen-xhigh"},
	} {
		t.Run(testCase.level, func(t *testing.T) {
			require.Equal(t, testCase.want, service.getMappedModelForThinkingLevel(account, "gemini-4-hydrogen", testCase.level))
		})
	}
}

func TestHydrogenClaudeEffortSelectsTier(t *testing.T) {
	for _, testCase := range []struct {
		name string
		body string
		want string
	}{
		{name: "default", body: `{}`, want: "high"},
		{name: "low effort", body: `{"output_config":{"effort":"low"}}`, want: "low"},
		{name: "medium effort", body: `{"output_config":{"effort":"medium"}}`, want: "medium"},
		{name: "high effort", body: `{"output_config":{"effort":"high"}}`, want: "high"},
		{name: "xhigh effort", body: `{"output_config":{"effort":"xhigh"}}`, want: "xhigh"},
		{name: "max effort", body: `{"output_config":{"effort":"max"}}`, want: "xhigh"},
		{name: "disabled takes precedence", body: `{"thinking":{"type":"disabled"},"output_config":{"effort":"xhigh"}}`, want: "low"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, gemini4ThinkingLevelFromClaudeBody("gemini-4-hydrogen", []byte(testCase.body)))
		})
	}
}
