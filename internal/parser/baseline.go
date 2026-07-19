package parser

import (
	"encoding/json"
	"os"
)

// Baseline is a snapshot of finding IDs from a previous run, used for comparison.
type Baseline struct {
	IDs map[string]bool `json:"ids"`
}

// NewBaseline creates a Baseline from the findings in a ParseResult.
func NewBaseline(r *ParseResult) *Baseline {
	b := &Baseline{IDs: make(map[string]bool)}
	for _, f := range r.Findings() {
		b.IDs[f.ID()] = true
	}
	return b
}

// Comparison holds the result of comparing current findings against a baseline.
type Comparison struct {
	New       []Finding // findings present now but not in baseline
	Resolved  []string  // finding IDs present in baseline but not now
	Unchanged int       // count of findings present in both
}

// Compare compares a ParseResult against this baseline and returns the comparison.
func (b *Baseline) Compare(r *ParseResult) *Comparison {
	c := &Comparison{}
	currentIDs := make(map[string]bool)
	for _, f := range r.Findings() {
		id := f.ID()
		currentIDs[id] = true
		if b.IDs[id] {
			c.Unchanged++
		} else {
			c.New = append(c.New, f)
		}
	}
	for id := range b.IDs {
		if !currentIDs[id] {
			c.Resolved = append(c.Resolved, id)
		}
	}
	return c
}

// Save writes the baseline as JSON to the given file path.
func (b *Baseline) Save(path string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// LoadBaseline reads a baseline from a JSON file.
func LoadBaseline(path string) (*Baseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b := &Baseline{}
	if err := json.Unmarshal(data, b); err != nil {
		return nil, err
	}
	if b.IDs == nil {
		b.IDs = make(map[string]bool)
	}
	return b, nil
}
