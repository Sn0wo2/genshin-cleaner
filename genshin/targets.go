package genshin

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hashicorp/go-version"
)

type ScanRule struct {
	Title   string        `json:"title"`
	Note    string        `json:"note"`
	Targets []fs.DirEntry `json:"targets"`
}

func CollectRules(g Genshin, editor bool) ([]ScanRule, error) {
	for _, path := range []string{g.Root, g.Data} {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}

	streaming := filepath.Join(g.Data, "StreamingAssets")
	webCaches := filepath.Join(g.Data, "webCaches")

	entries, err := os.ReadDir(webCaches)
	if err != nil {
		if _, statErr := os.Lstat(webCaches); !os.IsNotExist(err) || !os.IsNotExist(statErr) {
			return nil, err
		}
	}

	var scanErr error
	scan := func(cwd string, match func(rel string) bool) []fs.DirEntry {
		targets := []fs.DirEntry{}
		info, err := os.Lstat(cwd)
		if os.IsNotExist(err) {
			return targets
		}
		if err != nil {
			return nil
		}

		if info.Mode()&os.ModeSymlink != 0 {
			return targets
		}

		err = filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			rel, err := filepath.Rel(cwd, path)
			if err != nil {
				return nil
			}

			if match != nil && !match(filepath.ToSlash(rel)) {
				return nil
			}

			targets = append(targets, d)
			return nil
		})
		if err != nil {
			return nil
		}
		scanErr = errors.Join(scanErr, err)
		return targets
	}

	var versions []*version.Version
	for _, entry := range entries {
		if entry.IsDir() {
			if ver, err := version.NewVersion(entry.Name()); err == nil {
				versions = append(versions, ver)
			}
		}
	}
	slices.SortFunc(versions, func(a, b *version.Version) int {
		return a.Compare(b)
	})

	var latest *version.Version
	oldCaches := []fs.DirEntry{}
	if len(versions) > 0 {
		latest = versions[len(versions)-1]
		for _, cache := range versions[:len(versions)-1] {
			oldCaches = append(oldCaches, scan(filepath.Join(webCaches, cache.String()), nil)...)
		}
	}

	logs := scan(g.Data, func(rel string) bool {
		rel = strings.ToLower(rel)
		return rel == "persistent/downloaderror.log" || strings.HasSuffix(rel, ".tmp") || strings.HasSuffix(rel, ".bak")
	})
	logs = append(logs, scan(g.Root, func(rel string) bool {
		rel = strings.ToLower(rel)
		return strings.HasSuffix(rel, ".log") && !strings.Contains(rel, "/")
	})...)

	rules := []ScanRule{
		{
			Title:   "Cutscene videos (*.usm)",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scan(filepath.Join(streaming, "VideoAssets"), func(p string) bool { return strings.HasSuffix(p, ".usm") }),
		},
		{
			Title:   "BeyondUGC audio",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scan(filepath.Join(streaming, "AudioAssets", "BeyondUGC"), nil),
		},
		{
			Title:   "MusicGame audio",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scan(filepath.Join(streaming, "AudioAssets", "MusicGame"), nil),
		},
		{
			Title:   "Persistent on-demand CGs",
			Note:    "downloaded on demand, stays deleted",
			Targets: scan(filepath.Join(g.Data, "Persistent", "VideoAssets"), nil),
		},
		{
			Title:   "Old webCaches versions",
			Note:    "keeping " + latest.String(),
			Targets: oldCaches,
		},
		{
			Title:   "Logs & temp junk",
			Note:    "DownloadError.log, *.log, *.tmp, *.bak",
			Targets: logs,
		},
	}
	if editor {
		rules = append(rules, ScanRule{
			Title:   "BeyondAssistEditor (UGC editor)",
			Note:    "restorable via beyond_pkg_version",
			Targets: scan(filepath.Join(g.Root, "BeyondAssets", "BeyondAssistEditor"), nil),
		})
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return rules, nil
}

