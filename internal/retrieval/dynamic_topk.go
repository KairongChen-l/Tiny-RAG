package retrieval

// DynamicTopKConfig holds configuration for dynamic top-k selection.
// Instead of returning a fixed number of chunks, the algorithm analyzes
// the score distribution and cuts off at the "knee" of the curve,
// balancing recall against token cost and context noise.
type DynamicTopKConfig struct {
	MinK               int     // Minimum chunks to return (default: 2)
	MaxK               int     // Maximum chunks to return (default: 20)
	ScoreDropThreshold float32 // Relative score drop threshold to trigger cutoff (default: 0.3 = 30%)
	MinScoreThreshold  float32 // Absolute minimum score to keep a chunk (default: 0.5)
}

// DefaultDynamicTopKConfig returns a DynamicTopKConfig with sensible defaults.
func DefaultDynamicTopKConfig() DynamicTopKConfig {
	return DynamicTopKConfig{
		MinK:               2,
		MaxK:               20,
		ScoreDropThreshold: 0.3,
		MinScoreThreshold:  0.5,
	}
}

// DynamicTopKResult holds the output of a dynamic top-k selection.
type DynamicTopKResult struct {
	Chunks        []RetrievedChunk // Selected chunks
	OriginalCount int              // Number of input chunks
	SelectedCount int              // Number of selected chunks
	CutoffScore   float32          // Score at which cutoff happened
	Reason        string           // Reason for cutoff: "score_drop", "min_score", "max_k", "all_kept", "empty_input"
}

// DynamicTopKSelector selects chunks dynamically based on score distribution.
type DynamicTopKSelector struct {
	config DynamicTopKConfig
}

// NewDynamicTopKSelector creates a new DynamicTopKSelector with the given config.
func NewDynamicTopKSelector(config DynamicTopKConfig) *DynamicTopKSelector {
	if config.MinK <= 0 {
		config.MinK = 2
	}
	if config.MaxK <= 0 {
		config.MaxK = 20
	}
	if config.MinK > config.MaxK {
		config.MinK = config.MaxK
	}
	return &DynamicTopKSelector{config: config}
}

// Select dynamically selects chunks based on score distribution.
// Chunks are assumed to be sorted by score in descending order.
// The algorithm walks through consecutive chunks looking for a significant
// relative score drop (the "knee") while respecting MinK, MaxK, and
// MinScoreThreshold constraints.
func (s *DynamicTopKSelector) Select(chunks []RetrievedChunk) DynamicTopKResult {
	originalCount := len(chunks)

	if originalCount == 0 {
		return DynamicTopKResult{
			Chunks:        nil,
			OriginalCount: 0,
			SelectedCount: 0,
			CutoffScore:   0,
			Reason:        "empty_input",
		}
	}

	// Determine the upper bound of chunks we can consider.
	upperBound := originalCount
	if upperBound > s.config.MaxK {
		upperBound = s.config.MaxK
	}

	// Walk through chunks to find the cutoff point.
	cutoff := upperBound
	reason := "all_kept"
	var cutoffScore float32

	for i := 1; i < upperBound; i++ {
		prevScore := chunks[i-1].Score
		currScore := chunks[i].Score

		// Check absolute minimum score threshold.
		if currScore < s.config.MinScoreThreshold && i >= s.config.MinK {
			cutoff = i
			cutoffScore = currScore
			reason = "min_score"
			break
		}

		// Check relative score drop between consecutive chunks.
		if prevScore > 0 {
			drop := (prevScore - currScore) / prevScore
			if drop > s.config.ScoreDropThreshold && i >= s.config.MinK {
				cutoff = i
				cutoffScore = currScore
				reason = "score_drop"
				break
			}
		}
	}

	// If we exhausted the loop without an early cutoff, check if MaxK limited us.
	if reason == "all_kept" && upperBound < originalCount {
		reason = "max_k"
		cutoffScore = chunks[upperBound-1].Score
	} else if reason == "all_kept" && originalCount > 0 {
		cutoffScore = chunks[cutoff-1].Score
	}

	selected := make([]RetrievedChunk, cutoff)
	copy(selected, chunks[:cutoff])

	return DynamicTopKResult{
		Chunks:        selected,
		OriginalCount: originalCount,
		SelectedCount: cutoff,
		CutoffScore:   cutoffScore,
		Reason:        reason,
	}
}
