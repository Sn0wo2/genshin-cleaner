package main

import (
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

func collectRules(g genshin.Game, editor bool) []rule {
	streaming := filepath.Join(g.Data, "StreamingAssets")
	webCaches := filepath.Join(g.Data, "webCaches")

	entries, _ := os.ReadDir(webCaches)
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
			oldCaches = append(oldCaches, target{Path: directory, Size: totalSize(scanFiles(directory, nil))})
		}
	}

	logs := scanFiles(g.Data, func(rel string) bool {
		return rel == "Persistent/DownloadError.log" || strings.HasSuffix(rel, ".tmp") || strings.HasSuffix(rel, ".bak")
	})
	logs = append(logs, scanFiles(g.Root, func(rel string) bool {
		return strings.HasSuffix(rel, ".log") && !strings.Contains(rel, "/")
	})...)

	rules := []rule{
		{
			Title:   "Cutscene videos (*.usm)",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scanFiles(filepath.Join(streaming, "VideoAssets"), func(p string) bool { return strings.HasSuffix(p, ".usm") }),
		},
		{
			Title:   "BeyondUGC audio",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scanFiles(filepath.Join(streaming, "AudioAssets", "BeyondUGC"), nil),
		},
		{
			Title:   "MusicGame audio",
			Note:    "launcher re-downloads on base-resource updates",
			Targets: scanFiles(filepath.Join(streaming, "AudioAssets", "MusicGame"), nil),
		},
		{
			Title:   "Persistent on-demand CGs",
			Note:    "downloaded on demand, stays deleted",
			Targets: scanFiles(filepath.Join(g.Data, "Persistent", "VideoAssets"), nil),
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
			Targets: scanFiles(filepath.Join(g.Root, "BeyondAssets", "BeyondAssistEditor"), nil),
		})
	}
	return rules
}

func scanFiles(cwd string, match func(rel string) bool) []target {
	if info, err := os.Lstat(cwd); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return []target{}
	}

	var targets = []target{}

	_ = filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(cwd, path)
		if match != nil && !match(filepath.ToSlash(rel)) {
			return nil
		}
		if info, err := d.Info(); err == nil && !info.IsDir() {
			targets = append(targets, target{Path: path, Size: info.Size()})
		}
		return nil
	})
	return targets
}

func totalSize(targets []target) int64 {
	var total int64
	for _, t := range targets {
		total += t.Size
	}
	return total
}
