package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/sahilm/fuzzy"
)

func weightedFilter(term string, targets []string) []list.Rank {
	ranks := fuzzy.Find(term, targets)
	sort.SliceStable(ranks, func(i, j int) bool {
		left := scoreRank(term, targets[ranks[i].Index], ranks[i])
		right := scoreRank(term, targets[ranks[j].Index], ranks[j])
		if left == right {
			return ranks[i].Index < ranks[j].Index
		}
		return left < right
	})

	result := make([]list.Rank, len(ranks))
	for i, rank := range ranks {
		result[i] = list.Rank{
			Index:          rank.Index,
			MatchedIndexes: rank.MatchedIndexes,
		}
	}
	return result
}

func scoreRank(term, target string, rank fuzzy.Match) int {
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
