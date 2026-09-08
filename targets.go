package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sn0wo2/genshin-cleaner/genshin"
	"github.com/hashicorp/go-version"
)

type target struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type rule struct {
	Title   string   `json:"title"`
	Note    string   `json:"note"`
	Targets []target `json:"targets"`
}

func collectRules(g genshin.Game, editor bool) ([]rule, error) {
	for _, path := range []string{g.Root, g.Data} {
		if _, err := os.Stat(path); err != nil {
			return nil, err
		}
	}

	streaming := filepath.Join(g.Data, "StreamingAssets")
	webCaches := filepath.Join(g.Data, "webCaches")

	entries, err := os.ReadDir(webCaches)
	if err != nil {
		// On Windows, ReadDir can report a missing path for an existing file.
		if _, statErr := os.Lstat(webCaches); !os.IsNotExist(err) || !os.IsNotExist(statErr) {
			return nil, err
		}
	}
	var scanErr error
	scan := func(cwd string, match func(rel string) bool) []target {
		targets, err := scanFiles(cwd, match)
		scanErr = errors.Join(scanErr, err)
		return targets
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := version.NewVersion(entry.Name()); err == nil {
				versions = append(versions, entry.Name())
			}
		}
	}
	slices.SortFunc(versions, func(a, b string) int {
		va, _ := version.NewVersion(a)
		vb, _ := version.NewVersion(b)
		return va.Compare(vb)
	})

	latest := "none"
	oldCaches := []target{}
	if len(versions) > 0 {
		latest = versions[len(versions)-1]
		for _, cache := range versions[:len(versions)-1] {
			directory := filepath.Join(webCaches, cache)
			oldCaches = append(oldCaches, target{Path: directory, Size: totalSize(scan(directory, nil))})
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

	rules := []rule{
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
			Note:    "keeping " + latest,
			Targets: oldCaches,
		},
		{
			Title:   "Logs & temp junk",
			Note:    "DownloadError.log, *.log, *.tmp, *.bak",
			Targets: logs,
		},
	}
	if editor {
		rules = append(rules, rule{
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

func scanFiles(cwd string, match func(rel string) bool) ([]target, error) {
	targets := []target{}
	info, err := os.Lstat(cwd)
	if os.IsNotExist(err) {
		return targets, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return targets, nil
	}

	err = filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(cwd, path)
		if err != nil {
			return err
		}
		if match != nil && !match(filepath.ToSlash(rel)) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() {
			targets = append(targets, target{Path: path, Size: info.Size()})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return targets, nil
}

func totalSize(targets []target) int64 {
	var total int64
	for _, t := range targets {
		total += t.Size
	}
	return total
}
