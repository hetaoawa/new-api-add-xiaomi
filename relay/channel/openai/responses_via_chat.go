package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/helper"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func OaiChatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(nil, types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var chatResp dto.OpenAITextResponse
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responsesResp, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &chatResp.Usage, nil
}

func OaiChatToResponsesStreamHandler(c *gin.Context, _ *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	defer service.CloseResponseBodyGracefully(resp)

	responseID := helper.GetResponseID(c)
	createdAt := time.Now().Unix()
	model := ""
	usage := &dto.Usage{}
	assistantText := strings.Builder{}
	toolStates := make(map[int]dto.ToolCallResponse)
	messageItemID := "msg_0"
	sentCreated := false

	sendResponsesEvent := func(event dto.ResponsesStreamResponse) *types.NewAPIError {
		data, err := common.Marshal(event)
		if err != nil {
			return types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
		helper.ResponseChunkData(c, event, string(data))
		return nil
	}

	emitCreatedIfNeeded := func() *types.NewAPIError {
		if sentCreated {
			return nil
		}
		sentCreated = true
		return sendResponsesEvent(dto.ResponsesStreamResponse{
			Type: "response.created",
			Response: &dto.OpenAIResponsesResponse{
				ID:        responseID,
				Object:    "response",
				CreatedAt: int(createdAt),
				Model:     model,
			},
		})
	}

	helper.StreamScannerHandler(c, resp, nil, func(data string, sr *helper.StreamResult) {
		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			sr.Error(err)
			return
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Created != 0 {
			createdAt = chunk.Created
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}

		if err := emitCreatedIfNeeded(); err != nil {
			sr.Stop(err)
			return
		}

		for _, choice := range chunk.Choices {
			deltaText := choice.Delta.GetContentString()
			if deltaText != "" {
				assistantText.WriteString(deltaText)
				if err := sendResponsesEvent(dto.ResponsesStreamResponse{
					Type:         "response.output_text.delta",
					Delta:        deltaText,
					ItemID:       messageItemID,
					OutputIndex:  common.GetPointer(0),
					ContentIndex: common.GetPointer(0),
				}); err != nil {
					sr.Stop(err)
					return
				}
			}

			for _, tool := range choice.Delta.ToolCalls {
				idx := 0
				if tool.Index != nil {
					idx = *tool.Index
				}
				state := toolStates[idx]
				if state.ID == "" {
					state.ID = tool.ID
					if state.ID == "" {
						state.ID = fmt.Sprintf("call_%d", idx)
					}
					state.Type = "function"
				}
				if tool.Function.Name != "" {
					state.Function.Name = tool.Function.Name
				}
				if tool.Function.Arguments != "" {
					prevArgs := state.Function.Arguments
					state.Function.Arguments += tool.Function.Arguments
					if prevArgs == "" {
						if err := sendResponsesEvent(dto.ResponsesStreamResponse{
							Type: "response.output_item.added",
							Item: &dto.ResponsesOutput{
								Type:      "function_call",
								ID:        state.ID,
								CallId:    state.ID,
								Name:      state.Function.Name,
								Arguments: common.StringToByteSlice(state.Function.Arguments),
								Status:    "in_progress",
							},
							OutputIndex: common.GetPointer(idx),
						}); err != nil {
							sr.Stop(err)
							return
						}
					}
					if err := sendResponsesEvent(dto.ResponsesStreamResponse{
						Type:        "response.function_call_arguments.delta",
						ItemID:      state.ID,
						Delta:       tool.Function.Arguments,
						OutputIndex: common.GetPointer(idx),
					}); err != nil {
						sr.Stop(err)
						return
					}
				}
				toolStates[idx] = state
			}

			if choice.FinishReason != nil && *choice.FinishReason != "" {
				outputs := make([]dto.ResponsesOutput, 0, 1+len(toolStates))
				if assistantText.Len() > 0 {
					outputs = append(outputs, dto.ResponsesOutput{
						Type:   "message",
						ID:     messageItemID,
						Status: "completed",
						Role:   "assistant",
						Content: []dto.ResponsesOutputContent{
							{
								Type: "output_text",
								Text: assistantText.String(),
							},
						},
					})
				}
				for idx, state := range toolStates {
					output := dto.ResponsesOutput{
						Type:      "function_call",
						ID:        state.ID,
						CallId:    state.ID,
						Name:      state.Function.Name,
						Arguments: common.StringToByteSlice(state.Function.Arguments),
						Status:    "completed",
					}
					if err := sendResponsesEvent(dto.ResponsesStreamResponse{
						Type:        "response.function_call_arguments.done",
						ItemID:      state.ID,
						Delta:       state.Function.Arguments,
						OutputIndex: common.GetPointer(idx),
					}); err != nil {
						sr.Stop(err)
						return
					}
					if err := sendResponsesEvent(dto.ResponsesStreamResponse{
						Type:        "response.output_item.done",
						Item:        &output,
						OutputIndex: common.GetPointer(idx),
					}); err != nil {
						sr.Stop(err)
						return
					}
					outputs = append(outputs, output)
				}
				if err := sendResponsesEvent(dto.ResponsesStreamResponse{
					Type: "response.completed",
					Response: &dto.OpenAIResponsesResponse{
						ID:        responseID,
						Object:    "response",
						CreatedAt: int(createdAt),
						Model:     model,
						Output:    outputs,
						Usage:     usage,
					},
				}); err != nil {
					sr.Stop(err)
					return
				}
			}
		}
	})

	return usage, nil
}
