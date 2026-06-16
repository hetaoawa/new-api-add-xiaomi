package common

import "strings"

var (
	// OpenAIResponseOnlyModels is a list of models that are only available for OpenAI responses.
	OpenAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
		"mimo-v2-pro",
		"mimo-v2-flash",
		"mimo-v2-omni",
		"mimo-v2.5-pro",
		"mimo-v2.5",
	}
	ImageGenerationModels = []string{
		"dall-e-3",
		"dall-e-2",
		"gpt-image-1",
		"prefix:imagen-",
		"flux-",
		"flux.1-",
	}
	OpenAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
)

func IsOpenAIResponseOnlyModel(modelName string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if modelName == "" {
		return false
	}

	candidates := []string{modelName}
	if idx := strings.LastIndex(modelName, "/"); idx >= 0 && idx+1 < len(modelName) {
		candidates = append(candidates, modelName[idx+1:])
	}

	for _, name := range candidates {
		if isOpenAIResponseOnlyModelName(name) {
			return true
		}
	}
	return false
}

func isOpenAIResponseOnlyModelName(modelName string) bool {
	for _, m := range OpenAIResponseOnlyModels {
		candidate := strings.ToLower(strings.TrimSpace(m))
		if modelName == candidate {
			return true
		}
		if strings.HasPrefix(modelName, candidate+"-20") {
			return true
		}
	}
	return false
}

func IsImageGenerationModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range ImageGenerationModels {
		if strings.Contains(modelName, m) {
			return true
		}
		if strings.HasPrefix(m, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(m, "prefix:")) {
			return true
		}
	}
	return false
}

func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range OpenAITextModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}
