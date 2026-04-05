package recall

import "math"

// QueryResult holds the data needed to compute metrics for a single query.
type QueryResult struct {
	QueryID      string
	RetrievedIDs []string // ordered list of doc IDs from search results
	RelevantIDs  []string // ground truth relevant doc IDs
}

// RecallAtK computes the fraction of relevant documents found in the top-K results.
func RecallAtK(result QueryResult, k int) float64 {
	if len(result.RelevantIDs) == 0 {
		return 0
	}
	retrieved := truncate(result.RetrievedIDs, k)
	hits := uniqueIntersectionSize(retrieved, result.RelevantIDs)
	return float64(hits) / float64(len(result.RelevantIDs))
}

// PrecisionAtK computes the fraction of top-K results that are relevant.
func PrecisionAtK(result QueryResult, k int) float64 {
	if k == 0 {
		return 0
	}
	retrieved := truncate(result.RetrievedIDs, k)
	hits := intersectionSize(retrieved, result.RelevantIDs)
	return float64(hits) / float64(k)
}

// MRR computes the Mean Reciprocal Rank — 1/rank of the first relevant result.
func MRR(result QueryResult) float64 {
	relevantSet := toSet(result.RelevantIDs)
	for i, id := range result.RetrievedIDs {
		if relevantSet[id] {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// NDCGAtK computes Normalized Discounted Cumulative Gain at K.
// Binary relevance: relevant documents get gain=1, irrelevant get gain=0.
func NDCGAtK(result QueryResult, k int) float64 {
	dcg := dcgAtK(result.RetrievedIDs, result.RelevantIDs, k)
	idcg := idcgAtK(len(result.RelevantIDs), k)
	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

func dcgAtK(retrieved []string, relevant []string, k int) float64 {
	relevantSet := toSet(relevant)
	retrieved = truncate(retrieved, k)
	var sum float64
	for i, id := range retrieved {
		if relevantSet[id] {
			sum += 1.0 / math.Log2(float64(i)+2) // log2(rank+1), rank is 0-indexed so +2
		}
	}
	return sum
}

func idcgAtK(numRelevant int, k int) float64 {
	if numRelevant < k {
		k = numRelevant
	}
	var sum float64
	for i := 0; i < k; i++ {
		sum += 1.0 / math.Log2(float64(i)+2)
	}
	return sum
}

func truncate(ids []string, k int) []string {
	if len(ids) <= k {
		return ids
	}
	return ids[:k]
}

func toSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

func intersectionSize(a, b []string) int {
	bSet := toSet(b)
	count := 0
	for _, id := range a {
		if bSet[id] {
			count++
		}
	}
	return count
}

// uniqueIntersectionSize counts how many unique elements of 'a' also appear in 'b'.
// Unlike intersectionSize, duplicates in 'a' are only counted once.
func uniqueIntersectionSize(a, b []string) int {
	bSet := toSet(b)
	seen := make(map[string]bool, len(a))
	count := 0
	for _, id := range a {
		if bSet[id] && !seen[id] {
			seen[id] = true
			count++
		}
	}
	return count
}
