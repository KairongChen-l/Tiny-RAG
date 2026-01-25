package prompt

import (
	"strings"
)

// Truncator handles text truncation based on token limits.
type Truncator struct {
	tokenizer    Tokenizer
	maxTokens    int
	reserveTokens int // Tokens to reserve for response
}

// NewTruncator creates a new truncator.
func NewTruncator(tokenizer Tokenizer, maxTokens, reserveTokens int) *Truncator {
	return &Truncator{
		tokenizer:     tokenizer,
		maxTokens:     maxTokens,
		reserveTokens: reserveTokens,
	}
}

// TruncateText truncates text to fit within token limit.
func (t *Truncator) TruncateText(text string) string {
	available := t.maxTokens - t.reserveTokens
	if available <= 0 {
		return ""
	}

	tokens := t.tokenizer.Count(text)
	if tokens <= available {
		return text
	}

	// Binary search for the right length
	runes := []rune(text)
	low, high := 0, len(runes)

	for low < high {
		mid := (low + high + 1) / 2
		truncated := string(runes[:mid])
		if t.tokenizer.Count(truncated) <= available {
			low = mid
		} else {
			high = mid - 1
		}
	}

	if low == 0 {
		return ""
	}

	// Try to break at word boundary
	result := string(runes[:low])
	lastSpace := strings.LastIndex(result, " ")
	if lastSpace > low/2 {
		result = result[:lastSpace]
	}

	return result + "..."
}

// TruncateChunks truncates chunks to fit within token limit.
// Returns the truncated chunks and a boolean indicating if truncation occurred.
func (t *Truncator) TruncateChunks(chunks []string) ([]string, bool) {
	available := t.maxTokens - t.reserveTokens
	if available <= 0 {
		return nil, true
	}

	var result []string
	usedTokens := 0
	truncated := false

	for _, chunk := range chunks {
		chunkTokens := t.tokenizer.Count(chunk)
		if usedTokens+chunkTokens > available {
			truncated = true
			break
		}
		result = append(result, chunk)
		usedTokens += chunkTokens
	}

	return result, truncated
}

