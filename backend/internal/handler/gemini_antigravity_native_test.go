//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/gemini"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiNativeAntigravityListModelsWithoutMixedScheduling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(48)
	repository := &geminiAllowlistAccountRepoStub{gatewayModelsAccountRepoStub: gatewayModelsAccountRepoStub{
		byGroup: map[int64][]service.Account{
			groupID: {{
				ID:       1,
				Platform: service.PlatformAntigravity,
				Extra:    map[string]any{"mixed_scheduling": false},
				Credentials: map[string]any{"model_mapping": map[string]any{
					"gemini-4-hydrogen-xhigh":  "gemini-4-hydrogen-xhigh",
					"gemini-4-carbon-high":     "gemini-4-carbon-high",
					"gemini-group-custom-high": "gemini-4-hydrogen-high",
					"claude-sonnet-5-5-high":   "claude-sonnet-5-5-high",
				}},
			}},
			49: {{
				ID:          2,
				Platform:    service.PlatformAntigravity,
				Credentials: map[string]any{"model_mapping": map[string]any{"gemini-other-group": "gemini-other-group"}},
			}},
		},
	}}
	handler := &GatewayHandler{
		geminiCompatService: service.NewGeminiMessagesCompatService(repository, nil, nil, nil, nil, nil, nil, nil, nil),
	}
	for _, testCase := range []struct {
		name          string
		allowlist     service.GroupModelAllowlist
		expectedNames []string
	}{
		{name: "account mappings", expectedNames: []string{"models/gemini-4-carbon-high", "models/gemini-4-hydrogen-xhigh"}},
		{
			name:          "group allowlist",
			allowlist:     service.GroupModelAllowlist{Enabled: true, Models: []string{"gemini-group-custom-*"}},
			expectedNames: []string{"models/gemini-group-custom-high"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			requestContext, _ := gin.CreateTestContext(recorder)
			requestContext.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
			requestContext.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				GroupID: &groupID,
				Group:   &service.Group{ID: groupID, Platform: service.PlatformAntigravity, ModelAllowlist: testCase.allowlist},
			})
			handler.GeminiV1BetaListModels(requestContext)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			var response gemini.ModelsListResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			modelNames := make([]string, 0, len(response.Models))
			for _, model := range response.Models {
				modelNames = append(modelNames, model.Name)
				require.Contains(t, model.SupportedGenerationMethods, "generateContent")
				require.Contains(t, model.SupportedGenerationMethods, "streamGenerateContent")
			}
			require.NotContains(t, modelNames, "models/gemini-other-group")
			require.NotContains(t, modelNames, "models/claude-sonnet-5-5-high")
			if testCase.allowlist.Enabled {
				require.Equal(t, testCase.expectedNames, modelNames)
			} else {
				require.Subset(t, modelNames, testCase.expectedNames)
			}
		})
	}
}

func TestGeminiNativeAntigravityEndpointAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &GatewayHandler{}
	for _, platform := range []string{
		service.PlatformAntigravity,
		service.PlatformOpenAI,
		service.PlatformAnthropic,
		service.PlatformGrok,
	} {
		for _, endpoint := range []struct {
			name         string
			method       string
			parameter    string
			value        string
			handle       gin.HandlerFunc
			acceptedBody string
		}{
			{"detail", http.MethodGet, "model", "gemini-4-hydrogen-xhigh", handler.GeminiV1BetaGetModel, "models/gemini-4-hydrogen-xhigh"},
			{"generate", http.MethodPost, "modelAction", "/gemini-4-hydrogen-xhigh:generateContent", handler.GeminiV1BetaModels, "Request body is empty"},
			{"stream", http.MethodPost, "modelAction", "/gemini-4-hydrogen-xhigh:streamGenerateContent", handler.GeminiV1BetaModels, "Request body is empty"},
			{"count tokens", http.MethodPost, "modelAction", "/gemini-4-hydrogen-xhigh:countTokens", handler.GeminiV1BetaModels, "Request body is empty"},
		} {
			t.Run(platform+"/"+endpoint.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				requestContext, _ := gin.CreateTestContext(recorder)
				requestContext.Request = httptest.NewRequest(endpoint.method, "/v1beta/models/"+endpoint.value, nil)
				requestContext.Params = gin.Params{{Key: endpoint.parameter, Value: endpoint.value}}
				requestContext.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Group: &service.Group{Platform: platform}})
				requestContext.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1, Concurrency: 1})
				endpoint.handle(requestContext)
				if platform != service.PlatformAntigravity {
					require.Equal(t, http.StatusBadRequest, recorder.Code)
					require.Contains(t, recorder.Body.String(), "API key group platform is not gemini")
					return
				}
				expectedStatus := http.StatusBadRequest
				if endpoint.method == http.MethodGet {
					expectedStatus = http.StatusOK
				}
				require.Equal(t, expectedStatus, recorder.Code, recorder.Body.String())
				require.Contains(t, recorder.Body.String(), endpoint.acceptedBody)
			})
		}
	}
}
