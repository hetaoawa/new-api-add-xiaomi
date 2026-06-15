package xiaomi

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	channelconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := info.ChannelBaseUrl
	if baseURL == "" {
		baseURL = defaultMimoBaseURL
	}
	if specialPlan, ok := channelconstant.ChannelSpecialBases[baseURL]; ok {
		switch info.RelayFormat {
		case types.RelayFormatClaude:
			if specialPlan.ClaudeBaseURL == "" {
				return "", errors.New("xiaomi token plan claude base url is empty")
			}
			return fmt.Sprintf("%s/v1/messages", specialPlan.ClaudeBaseURL), nil
		default:
			if info.RelayMode == relayconstant.RelayModeAudioSpeech {
				if specialPlan.OpenAIBaseURL == "" {
					return "", errors.New("xiaomi token plan openai base url is empty")
				}
				return fmt.Sprintf("%s/chat/completions", specialPlan.OpenAIBaseURL), nil
			}
			if specialPlan.OpenAIBaseURL == "" {
				return "", errors.New("xiaomi token plan openai base url is empty")
			}
			return relaycommon.GetFullRequestURL(specialPlan.OpenAIBaseURL, info.RequestURLPath, info.ChannelType), nil
		}
	}
	info.ChannelBaseUrl = baseURL
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		if info.RelayFormat == types.RelayFormatClaude {
			return fmt.Sprintf("%s/anthropic/v1/messages", baseURL), nil
		}
		return a.Adaptor.GetRequestURL(info)
	}
	return fmt.Sprintf("%s/v1/chat/completions", baseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if info.RelayFormat == types.RelayFormatClaude {
		adaptor := claude.Adaptor{}
		if err := adaptor.SetupRequestHeader(c, req, info); err != nil {
			return err
		}
		req.Set("api-key", info.ApiKey)
		req.Del("x-api-key")
		return nil
	}
	if err := a.Adaptor.SetupRequestHeader(c, req, info); err != nil {
		return err
	}
	req.Set("api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := claude.Adaptor{}
	return adaptor.ConvertClaudeRequest(c, info, req)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, fmt.Errorf("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return a.Adaptor.ConvertAudioRequest(c, info, request)
	}
	return convertTTSRequest(c, request)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation ||
		info.RelayMode == relayconstant.RelayModeImagesEdits {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayFormat == types.RelayFormatClaude {
		adaptor := claude.Adaptor{}
		return adaptor.DoResponse(c, resp, info)
	}
	if info.RelayMode == relayconstant.RelayModeAudioSpeech {
		return handleTTSResponse(c, resp, info)
	}
	return a.Adaptor.DoResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
