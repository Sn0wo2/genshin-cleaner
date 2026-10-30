package genshin

import (
	"io/fs"
	"slices"
)

type Selection struct {
	Keep *[]string `json:"keep,omitempty"`
	Drop *[]string `json:"drop,omitempty"`
}

func (s Selection) Apply(rules []ScanRule) []ScanRule {
	out := make([]ScanRule, 0, len(rules))
	for _, r := range rules {
		if s.Keep != nil || s.Drop != nil {
			targets := make(map[string]fs.DirEntry, len(r.Targets))
			for path, entry := range r.Targets {
				if (s.Keep == nil || slices.Contains(*s.Keep, r.Id) || slices.Contains(*s.Keep, path)) &&
					(s.Drop == nil || !slices.Contains(*s.Drop, r.Id) && !slices.Contains(*s.Drop, path)) {
					targets[path] = entry
				}
			}

			r.Targets = targets
		}

		if len(r.Targets) > 0 {
			out = append(out, r)
		}
	}

	return out
}
