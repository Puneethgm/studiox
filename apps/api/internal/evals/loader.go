package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Split names the two case-set partitions — doc §8.1's dev/held-out
// split. Dev cases are safe to look at while iterating on the pipeline;
// held-out cases are only run right before shipping, so they stay a
// trustworthy check that the pipeline wasn't tuned to them.
type Split string

const (
	SplitDev     Split = "dev"
	SplitHeldout Split = "heldout"
)

// LoadCases reads every *.json file in dir (non-recursive) as a Case,
// sorted by filename for a deterministic run order. It returns an error
// naming the offending file on the first malformed fixture, rather than
// silently skipping it — a bad fixture should fail loudly, not quietly
// shrink the case set.
func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read case dir %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	cases := make([]Case, 0, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path) // #nosec G304 -- offline eval CLI tool; dir comes from the fixed internal/evals/cases/ layout and name from this same directory's own os.ReadDir listing, never from an HTTP request
		if err != nil {
			return nil, fmt.Errorf("read case file %s: %w", path, err)
		}
		var c Case
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse case file %s: %w", path, err)
		}
		if c.ID == "" {
			return nil, fmt.Errorf("case file %s: missing required \"id\" field", path)
		}
		if c.CustomerMessage == "" {
			return nil, fmt.Errorf("case file %s (id=%s): missing required \"customer_message\" field", path, c.ID)
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// LoadCaseSetFromPath loads both the dev and held-out splits from root
// (a directory containing dev/ and heldout/ subdirectories, as laid out
// under internal/evals/cases/).
func LoadCaseSetFromPath(root string) (dev, heldout []Case, err error) {
	dev, err = LoadCases(filepath.Join(root, string(SplitDev)))
	if err != nil {
		return nil, nil, err
	}
	heldout, err = LoadCases(filepath.Join(root, string(SplitHeldout)))
	if err != nil {
		return nil, nil, err
	}
	return dev, heldout, nil
}
