package tui

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/list"
	"github.com/sahilm/fuzzy"
)

func weightedFilter(term string, targets []string) []list.Rank {
	if strings.TrimSpace(term) == "" {
		return nil
	}

	matches := collectMatches(term, targets)
	sort.SliceStable(matches, func(i, j int) bool {
		left := scoreRank(term, targets[matches[i].Index], matches[i])
		right := scoreRank(term, targets[matches[j].Index], matches[j])
		if left == right {
			return matches[i].Index < matches[j].Index
		}
		return left < right
	})

	result := make([]list.Rank, len(matches))
	for i, rank := range matches {
		result[i] = list.Rank{
			Index:          rank.Index,
			MatchedIndexes: rank.MatchedIndexes,
		}
	}
	return result
}

func collectMatches(term string, targets []string) []fuzzy.Match {
	lowerTerm := strings.ToLower(term)
	normalizedTerm := normalizeFilterText(term)
	matchesByIndex := make(map[int]fuzzy.Match, len(targets))

	for index, target := range targets {
		lowerTarget := strings.ToLower(target)
		switch {
		case strings.Contains(lowerTarget, lowerTerm):
			start := strings.Index(lowerTarget, lowerTerm)
			matchesByIndex[index] = fuzzy.Match{
				Str:            target,
				Index:          index,
				MatchedIndexes: contiguousIndexes(start, len([]rune(term))),
			}
		case normalizedTerm != "" && strings.Contains(normalizeFilterText(target), normalizedTerm):
			ranked := fuzzy.Find(term, []string{target})
			match := fuzzy.Match{Str: target, Index: index}
			if len(ranked) > 0 {
				match.MatchedIndexes = ranked[0].MatchedIndexes
			}
			matchesByIndex[index] = match
		}
	}

	for _, match := range fuzzy.Find(term, targets) {
		if _, exists := matchesByIndex[match.Index]; exists {
			continue
		}
		matchesByIndex[match.Index] = match
	}

	result := make([]fuzzy.Match, 0, len(matchesByIndex))
	for _, match := range matchesByIndex {
		result = append(result, match)
	}
	return result
}

func scoreRank(term, target string, rank fuzzy.Match) int {
	lowerTerm := strings.ToLower(term)
	lowerTarget := strings.ToLower(target)
	normalizedTerm := normalizeFilterText(term)
	normalizedTarget := normalizeFilterText(target)

	switch {
	case lowerTerm != "" && strings.HasPrefix(lowerTarget, lowerTerm):
		return -3000
	case lowerTerm != "" && strings.Contains(lowerTarget, lowerTerm):
		return -2000 + strings.Index(lowerTarget, lowerTerm)
	case normalizedTerm != "" && strings.HasPrefix(normalizedTarget, normalizedTerm):
		return -1500
	case normalizedTerm != "" && strings.Contains(normalizedTarget, normalizedTerm):
		return -1200
	}

	labelEnd := strings.Index(target, " ")
	if labelEnd < 0 {
		labelEnd = len(target)
	}
	valueStart := strings.Index(target, " ")
	labelBias := 300
	valueBias := 600
	metaBias := 1000
	if len(rank.MatchedIndexes) == 0 {
		return metaBias
	}
	first := rank.MatchedIndexes[0]
	bias := metaBias
	switch {
	case first < labelEnd:
		bias = labelBias
	case valueStart >= 0 && first < valueStart+1+len(term)*4:
		bias = valueBias
	}
	return bias + rank.Index
}

func normalizeFilterText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func contiguousIndexes(start, count int) []int {
	if start < 0 || count <= 0 {
		return nil
	}
	result := make([]int, count)
	for i := 0; i < count; i++ {
		result[i] = start + i
	}
	return result
}
