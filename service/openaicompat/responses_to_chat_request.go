package openaicompat

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
)

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, fmt.Errorf("model is required")
	}

	out := &dto.GeneralOpenAIRequest{
		Model:                req.Model,
		Stream:               req.Stream,
		MaxCompletionTokens:  req.MaxOutputTokens,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		Metadata:             req.Metadata,
		Store:                req.Store,
		PromptCacheRetention: req.PromptCacheRetention,
		Reasoning:            nil,
	}
	if len(req.PromptCacheKey) > 0 {
		out.PromptCacheKey = common.JsonRawMessageToString(req.PromptCacheKey)
	}

	if req.Reasoning != nil && req.Reasoning.Effort != "" {
		out.ReasoningEffort = normalizeResponsesReasoningEffort(req.Reasoning.Effort)
	}
	if req.StreamOptions != nil {
		out.StreamOptions = req.StreamOptions
	}

	if len(req.Instructions) > 0 {
		out.Messages = append(out.Messages, dto.Message{
			Role:    "system",
			Content: string(req.Instructions),
		})
	}

	inputs := req.ParseInput()
	if len(inputs) == 0 {
		out.Messages = append(out.Messages, dto.Message{
			Role:    "user",
			Content: "",
		})
	} else {
		mediaContents := make([]dto.MediaContent, 0, len(inputs))
		for _, input := range inputs {
			switch input.Type {
			case "input_text":
				mediaContents = append(mediaContents, dto.MediaContent{
					Type: dto.ContentTypeText,
					Text: input.Text,
				})
			case "input_image":
				mediaContents = append(mediaContents, dto.MediaContent{
					Type: dto.ContentTypeImageURL,
					ImageUrl: &dto.MessageImageUrl{
						Url:    input.ImageUrl,
						Detail: lo.If(input.Detail != "", input.Detail).Else("auto"),
					},
				})
			case "input_file":
				mediaContents = append(mediaContents, dto.MediaContent{
					Type: dto.ContentTypeFile,
					File: &dto.MessageFile{
						FileId: input.FileUrl,
					},
				})
			}
		}

		msg := dto.Message{Role: "user"}
		if len(mediaContents) == 1 && mediaContents[0].Type == dto.ContentTypeText {
			msg.SetStringContent(mediaContents[0].Text)
		} else {
			msg.SetMediaContent(mediaContents)
		}
		out.Messages = append(out.Messages, msg)
	}

	if len(req.Tools) > 0 {
		for _, tool := range req.GetToolsMap() {
			toolType := strings.TrimSpace(fmt.Sprintf("%v", tool["type"]))
			if toolType != "function" {
				continue
			}
			name := strings.TrimSpace(fmt.Sprintf("%v", tool["name"]))
			if name == "" {
				continue
			}
			out.Tools = append(out.Tools, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        name,
					Description: strings.TrimSpace(fmt.Sprintf("%v", tool["description"])),
					Parameters:  tool["parameters"],
				},
			})
		}
	}

	if len(req.ToolChoice) > 0 {
		var toolChoice any
		toolChoiceRaw := strings.TrimSpace(string(req.ToolChoice))
		if toolChoiceRaw != "" {
			switch toolChoiceRaw {
			case `"auto"`:
				toolChoice = "auto"
			case `"none"`:
				toolChoice = "none"
			default:
				var obj map[string]any
				if err := common.Unmarshal(req.ToolChoice, &obj); err == nil && len(obj) > 0 {
					if obj["type"] == "function" && obj["name"] != nil {
						toolChoice = map[string]any{
							"type": "function",
							"function": map[string]any{
								"name": obj["name"],
							},
						}
					} else {
						toolChoice = obj
					}
				}
			}
		}
		if toolChoice != nil {
			out.ToolChoice = toolChoice
		}
	}

	return out, nil
}
