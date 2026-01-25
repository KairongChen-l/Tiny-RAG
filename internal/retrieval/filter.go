package retrieval

// FilterOptions holds metadata filtering options.
type FilterOptions struct {
	// Exact match filters
	Exact map[string]string

	// Contains filters (for array fields)
	Contains map[string][]string
}

// NewFilterOptions creates a new FilterOptions.
func NewFilterOptions() *FilterOptions {
	return &FilterOptions{
		Exact:    make(map[string]string),
		Contains: make(map[string][]string),
	}
}

// AddExact adds an exact match filter.
func (f *FilterOptions) AddExact(key, value string) *FilterOptions {
	f.Exact[key] = value
	return f
}

// AddContains adds a contains filter.
func (f *FilterOptions) AddContains(key string, values ...string) *FilterOptions {
	f.Contains[key] = values
	return f
}

// ToMetadataFilter converts to simple metadata filter map.
func (f *FilterOptions) ToMetadataFilter() map[string]string {
	return f.Exact
}

// MatchesChunk checks if a chunk matches the filter options.
func (f *FilterOptions) MatchesChunk(metadata map[string]string) bool {
	// Check exact matches
	for key, expected := range f.Exact {
		if actual, ok := metadata[key]; !ok || actual != expected {
			return false
		}
	}

	// Contains filters not implemented for simple metadata
	// Would require array-typed metadata

	return true
}

