package local

import (
	"fmt"
	"math"
	"ngareader/internal/nga"
)

type ReadingPosition struct {
	PID      int64   `json:"pid,omitempty"`
	Floor    int     `json:"floor"`
	Fraction float32 `json:"fraction,omitempty"`
}

func normalizedReading(r Reading) Reading {
	r.Page = min(1000000, max(1, r.Page))
	if r.UID < 0 {
		r.UID = 0
		r.Scoped = false
	}
	if p := r.Position; p != nil {
		if !r.Scoped || p.PID < 0 || p.Floor < 0 || p.Floor > 100000000 || math.IsNaN(float64(p.Fraction)) || math.IsInf(float64(p.Fraction), 0) {
			r.Position = nil
		} else {
			copy := *p
			copy.Fraction = min(1, max(0, copy.Fraction))
			r.Position = &copy
		}
	}
	return r
}
func normalizeReadings(history []Reading) []Reading {
	out := make([]Reading, 0, min(100, len(history)))
	seen := map[string]bool{}
	for _, r := range history {
		if r.Topic.ID <= 0 {
			continue
		}
		r = normalizedReading(r)
		key := fmt.Sprintf("%d/%d/%t", r.Topic.ID, r.UID, r.Scoped)
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
		if len(out) == 100 {
			break
		}
	}
	return out
}
func FindReading(history []Reading, topic, uid int64) (Reading, bool) {
	for _, r := range history {
		if r.Topic.ID == topic && r.Scoped && r.UID == uid {
			return normalizedReading(r), true
		}
	}
	for _, r := range history {
		if r.Topic.ID == topic && !r.Scoped {
			r.Position = nil
			return normalizedReading(r), true
		}
	}
	return Reading{}, false
}
func PutReading(history []Reading, incoming Reading) []Reading {
	incoming = normalizedReading(incoming)
	if incoming.Topic.ID <= 0 {
		return normalizeReadings(history)
	}
	out := []Reading{incoming}
	for _, r := range history {
		if r.Topic.ID == incoming.Topic.ID && (!r.Scoped || r.UID == incoming.UID && r.Scoped == incoming.Scoped) {
			continue
		}
		out = append(out, r)
	}
	return normalizeReadings(out)
}
func ReadStatus(history []Reading, topic nga.Topic, uid int64) (seen, changed bool) {
	r, ok := FindReading(history, topic.ID, uid)
	if !ok || !r.Scoped && uid != 0 {
		return false, false
	}
	return true, r.RepliesKnown && topic.Replies > r.Topic.Replies
}
func VisibleReadings(history []Reading, uid int64) []Reading {
	out := []Reading{}
	seen := map[int64]bool{}
	for _, r := range history {
		if (!r.Scoped || r.UID == uid) && !seen[r.Topic.ID] {
			out = append(out, r)
			seen[r.Topic.ID] = true
		}
	}
	return out
}
