package prompt

import (
	"regexp"
	"strconv"
)

// CitationPattern is the regex pattern for matching citations.
var CitationPattern = regexp.MustCompile(`\[citation:(\d+)\]`)

// ParseCitations extracts citation numbers from text.
func ParseCitations(text string) []int {
	matches := CitationPattern.FindAllStringSubmatch(text, -1)
	
	seen := make(map[int]bool)
	var citations []int

	for _, match := range matches {
		if len(match) > 1 {
			if num, err := strconv.Atoi(match[1]); err == nil {
				if !seen[num] {
					seen[num] = true
					citations = append(citations, num)
				}
			}
		}
	}

	return citations
}

// FormatCitation formats a citation number.
func FormatCitation(num int) string {
	return "[citation:" + strconv.Itoa(num) + "]"
}

// ReplaceCitations replaces [citation:N] with a custom format.
func ReplaceCitations(text string, formatter func(int) string) string {
	return CitationPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := CitationPattern.FindStringSubmatch(match)
		if len(submatches) > 1 {
			if num, err := strconv.Atoi(submatches[1]); err == nil {
				return formatter(num)
			}
		}
		return match
	})
}

