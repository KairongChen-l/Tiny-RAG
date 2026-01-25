// Package tokenizer provides token counting functionality.
package tokenizer

import (
	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer counts tokens for text using tiktoken.
type Tokenizer struct {
	encoding *tiktoken.Tiktoken
}

// New creates a new tokenizer for the specified model.
func New(model string) (*Tokenizer, error) {
	encoding, err := tiktoken.EncodingForModel(model)
	if err != nil {
		// Fall back to cl100k_base (GPT-4 encoding)
		encoding, err = tiktoken.GetEncoding("cl100k_base")
		if err != nil {
			return nil, err
		}
	}
	return &Tokenizer{encoding: encoding}, nil
}

// Count returns the number of tokens in the text.
func (t *Tokenizer) Count(text string) int {
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// Encode returns the token IDs for the text.
func (t *Tokenizer) Encode(text string) []int {
	return t.encoding.Encode(text, nil, nil)
}

// Decode converts token IDs back to text.
func (t *Tokenizer) Decode(tokens []int) string {
	return t.encoding.Decode(tokens)
}

// Truncate truncates text to fit within maxTokens.
func (t *Tokenizer) Truncate(text string, maxTokens int) string {
	tokens := t.encoding.Encode(text, nil, nil)
	if len(tokens) <= maxTokens {
		return text
	}
	return t.encoding.Decode(tokens[:maxTokens])
}

