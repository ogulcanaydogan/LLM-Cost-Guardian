package tokenizer

import (
	"fmt"
	"strings"

	"github.com/tiktoken-go/tokenizer"
)

// encodingForModel maps OpenAI model names to tiktoken encoding names.
var encodingForModel = map[string]string{
	"gpt-4o":        "o200k_base",
	"gpt-4o-mini":   "o200k_base",
	"o1":            "o200k_base",
	"o1-mini":       "o200k_base",
	"o3-mini":       "o200k_base",
	"gpt-4-turbo":   "cl100k_base",
	"gpt-4":         "cl100k_base",
	"gpt-3.5-turbo": "cl100k_base",
}

// CountTokens returns the token count for the given text and model.
// For OpenAI models it uses tiktoken; for others it uses character-based estimation.
func CountTokens(text string, provider string, model string) (int64, error) {
	if provider == "openai" {
		return countOpenAI(text, model)
	}
	return estimateTokens(text), nil
}

// countOpenAI uses tiktoken to count tokens for OpenAI models.
func countOpenAI(text string, model string) (int64, error) {
	encName, ok := encodingForModel[model]
	if !ok {
		// Fall back to cl100k_base for unknown OpenAI models
		encName = "cl100k_base"
	}

	var enc tokenizer.Codec
	var err error

	switch encName {
	case "o200k_base":
		enc, err = tokenizer.Get(tokenizer.O200kBase)
	case "cl100k_base":
		enc, err = tokenizer.Get(tokenizer.Cl100kBase)
	default:
		return estimateTokens(text), nil
	}

	if err != nil {
		return 0, fmt.Errorf("load encoding %s: %w", encName, err)
	}

	ids, _, err := enc.Encode(text)
	if err != nil {
		return 0, fmt.Errorf("encode text: %w", err)
	}

	return int64(len(ids)), nil
}

// estimateTokens uses character-based estimation (4 chars per token on average).
// This is used for non-OpenAI providers or as a fallback.
func estimateTokens(text string) int64 {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return 0
	}
	tokens := (len(text) + 3) / 4 // ceiling division by 4
	return int64(tokens)
}

// CountChatTokens counts tokens for a series of chat messages (OpenAI format).
// Each message adds ~4 tokens of overhead for role/formatting.
func CountChatTokens(messages []map[string]string, provider string, model string) (int64, error) {
	var total int64
	for _, msg := range messages {
		total += 4 // message overhead (role, formatting)
		for _, value := range msg {
			count, err := CountTokens(value, provider, model)
			if err != nil {
				return 0, err
			}
			total += count
		}
	}
	total += 2 // assistant reply priming
	return total, nil
}
