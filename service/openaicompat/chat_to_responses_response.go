package openaicompat

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
)

func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse) (*dto.OpenAIResponsesResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	out := &dto.OpenAIResponsesResponse{
		ID:                resp.Id,
		Object:            "response",
		CreatedAt:         normalizeResponsesCreatedAt(resp.Created),
		Model:             resp.Model,
		Output:            make([]dto.ResponsesOutput, 0, len(resp.Choices)),
		ParallelToolCalls: false,
		Store:             false,
		Temperature:       0,
		TopP:              0,
		Usage:             &resp.Usage,
	}

	if out.CreatedAt == 0 {
		out.CreatedAt = int(time.Now().Unix())
	}

	for _, choice := range resp.Choices {
		output := dto.ResponsesOutput{
			Type:   "message",
			ID:     fmt.Sprintf("msg_%d", choice.Index),
			Status: "completed",
			Role:   "assistant",
		}

		toolCalls := choice.Message.ParseToolCalls()
		if len(toolCalls) > 0 {
			for _, toolCall := range toolCalls {
				out.Output = append(out.Output, dto.ResponsesOutput{
					Type:      "function_call",
					ID:        toolCall.ID,
					CallId:    toolCall.ID,
					Name:      toolCall.Function.Name,
					Arguments: common.StringToJsonRawMessage(toolCall.Function.Arguments),
					Status:    "completed",
				})
			}
			continue
		}

		content := choice.Message.StringContent()
		if content != "" {
			output.Content = append(output.Content, dto.ResponsesOutputContent{
				Type: "output_text",
				Text: content,
			})
		}
		out.Output = append(out.Output, output)
	}

	return out, nil
}

func normalizeResponsesCreatedAt(created any) int {
	switch v := created.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}
