package provider

// anthropic_messages.go — the Anthropic Messages protocol as a headless
// HTTP stream transport: POST {base}/v1/messages, x-api-key auth, SSE
// events. Powers the zai runtime — z.ai serves the GLM Coding Plan over
// this protocol at https://api.z.ai/api/anthropic, so a zai-bound agent
// speaks the same wire format as a Claude Code CLI on the same plan.
//
// The OpenAI-compat transport (openai_compat.go) covers every other
// OpenAI-shaped endpoint; Entry.Transport selects between the two and
// provider.NewStreamFnFor is the single place a turn runner asks for a
// stream fn — no transport list anywhere else.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Northlatch-Labs-LLC/hivex/internal/bot"
	"github.com/Northlatch-Labs-LLC/hivex/internal/config"
)

const anthropicAPIVersion = "2023-06-01"

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Stream    bool               `json:"stream"`
	Messages  []anthropicMessage `json:"messages"`
}

// botMsgsToAnthropic maps the harness message list onto the Messages wire
// shape: system messages are hoisted into the top-level system field (the
// protocol has no in-band system role), everything else passes through as
// user/assistant turns.
func botMsgsToAnthropic(msgs []bot.Message) (system string, out []anthropicMessage) {
	var sysParts []string
	for _, m := range msgs {
		switch {
		case m.Role == "system":
			if t := strings.TrimSpace(m.Content); t != "" {
				sysParts = append(sysParts, t)
			}
		case m.Role == "user" || m.Role == "assistant":
			if t := strings.TrimSpace(m.Content); t != "" {
				out = append(out, anthropicMessage{Role: m.Role, Content: t})
			}
		}
	}
	return strings.Join(sysParts, "\n\n"), out
}

// anthropicDefaults mirrors the OpenAI-compat kind→endpoint table so the
// ctx-aware constructor resolves compile-time defaults without duplicating
// the registration.
var (
	anthropicDefaultsMu sync.RWMutex
	anthropicDefaults   = map[string]openAICompatKindDefaults{}
)

func registerAnthropicDefaults(kind, baseURL, model string) {
	anthropicDefaultsMu.Lock()
	defer anthropicDefaultsMu.Unlock()
	anthropicDefaults[kind] = openAICompatKindDefaults{baseURL, model}
}

func anthropicDefaultsFor(kind string) (string, string) {
	anthropicDefaultsMu.RLock()
	defer anthropicDefaultsMu.RUnlock()
	d := anthropicDefaults[kind]
	return d.baseURL, d.model
}

// NewAnthropicMessagesStreamFn registers the kind's defaults and returns a
// StreamFn whose HTTP request lifetime is the background context. Turn
// runners should use NewStreamFnFor so cancellation propagates.
func NewAnthropicMessagesStreamFn(kind, defaultBaseURL, defaultModel string) func(string) bot.StreamFn {
	registerAnthropicDefaults(kind, defaultBaseURL, defaultModel)
	return func(string) bot.StreamFn {
		return func(msgs []bot.Message, tools []bot.BotTool) <-chan bot.StreamChunk {
			ch := make(chan bot.StreamChunk, 64)
			go runAnthropicMessagesStream(context.Background(), ch, kind, msgs, "", nil)
			return ch
		}
	}
}

// runAnthropicMessagesStream executes one Messages-protocol turn against
// the kind's endpoint and translates the SSE stream into StreamChunks.
func runAnthropicMessagesStream(
	parentCtx context.Context,
	ch chan<- bot.StreamChunk,
	kind string,
	msgs []bot.Message,
	modelOverride string,
	_ []bot.BotTool,
) {
	defer close(ch)

	baseURL, model := func() (string, string) {
		defBase, defModel := anthropicDefaultsFor(kind)
		return config.ResolveProviderEndpoint(kind, defBase, defModel)
	}()
	if trimmed := strings.TrimSpace(modelOverride); trimmed != "" {
		model = trimmed
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/v1/messages"

	system, wireMsgs := botMsgsToAnthropic(msgs)
	body := anthropicRequest{
		Model:     model,
		MaxTokens: 8192,
		System:    system,
		Stream:    true,
		Messages:  wireMsgs,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		ch <- bot.StreamChunk{Type: "error", Content: fmt.Sprintf("anthropic (%s): marshal request: %v", kind, err)}
		return
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		ch <- bot.StreamChunk{Type: "error", Content: fmt.Sprintf("anthropic (%s): build request: %v", kind, err)}
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	if key := resolveOpenAICompatAPIKey(kind); key != "" {
		// The protocol's native header is x-api-key; some gateways (z.ai
		// included) also accept Authorization: Bearer. Send both so the
		// same credential works everywhere.
		req.Header.Set("x-api-key", key)
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := httpClientForOpenAICompat().Do(req)
	if err != nil {
		ch <- bot.StreamChunk{Type: "error", Content: fmt.Sprintf("anthropic (%s): connect %s: %v", kind, endpoint, err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		ch <- bot.StreamChunk{
			Type:    "error",
			Content: fmt.Sprintf("anthropic (%s): HTTP %d from %s: %s", kind, resp.StatusCode, endpoint, strings.TrimSpace(string(errBody))),
		}
		return
	}

	parseAnthropicSSEStream(ch, kind, resp.Body)
}

type anthropicSSEEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// parseAnthropicSSEStream translates the Messages-protocol SSE vocabulary
// (message_start, content_block_delta, message_delta, message_stop, error)
// into hivex StreamChunks. Pulled out for direct testing.
func parseAnthropicSSEStream(ch chan<- bot.StreamChunk, kind string, body io.Reader) {
	reader := bufio.NewReaderSize(body, 64<<10)
	var inTokens, outTokens int
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			trimmed := strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(trimmed, "data: ") {
				var ev anthropicSSEEvent
				if jerr := json.Unmarshal([]byte(strings.TrimPrefix(trimmed, "data: ")), &ev); jerr == nil {
					switch {
					case ev.Error != nil:
						ch <- bot.StreamChunk{Type: "error", Content: fmt.Sprintf("anthropic (%s): %s: %s", kind, ev.Error.Type, ev.Error.Message)}
					case ev.Type == "content_block_delta":
						switch ev.Delta.Type {
						case "text_delta":
							if ev.Delta.Text != "" {
								ch <- bot.StreamChunk{Type: "text", Content: ev.Delta.Text}
							}
						case "thinking_delta":
							if ev.Delta.Text != "" {
								ch <- bot.StreamChunk{Type: "thinking", Content: ev.Delta.Text}
							}
						}
					case ev.Type == "message_start":
						inTokens = ev.Usage.InputTokens
					case ev.Type == "message_delta":
						outTokens = ev.Usage.OutputTokens
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	if inTokens > 0 || outTokens > 0 {
		ch <- bot.StreamChunk{Type: "usage", InputTokens: inTokens, OutputTokens: outTokens}
	}
}
