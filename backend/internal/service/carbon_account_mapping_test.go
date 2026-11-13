package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCarbonPreservesCustomAccountMappings(t *testing.T) {
	testCases := []struct {
		name          string
		mapping       map[string]any
		expectedModel string
	}{
		{
			name:          "existing unrelated mapping receives Carbon passthrough",
			mapping:       map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"},
			expectedModel: "gemini-4-carbon",
		},
		{
			name:          "explicit Carbon mapping is preserved",
			mapping:       map[string]any{"gemini-4-carbon": "custom-carbon"},
			expectedModel: "custom-carbon",
		},
		{
			name:          "wildcard mapping is preserved",
			mapping:       map[string]any{"gemini-*": "custom-gemini"},
			expectedModel: "custom-gemini",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			account := &Account{
				Platform:    PlatformAntigravity,
				Credentials: map[string]any{"model_mapping": testCase.mapping},
			}
			require.True(t, account.IsModelSupported("gemini-4-carbon"))
			require.Equal(t, testCase.expectedModel, account.GetMappedModel("gemini-4-carbon"))
		})
	}
}
