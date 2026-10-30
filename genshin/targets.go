package genshin

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Sn0wo2/genshin-cleaner/utils"
	"github.com/hashicorp/go-version"
)

type ScanRule struct {
	Id      string                 `json:"id"`
	Params  []string               `json:"params,omitempty"`
	Targets map[string]fs.DirEntry `json:"targets"`
}

type CutsceneRule struct {
	Id       string
	Keywords []string
}

var cutsceneRules = []CutsceneRule{
	{"cutsceneBoy", []string{"boy"}},
	{"cutsceneGirl", []string{"girl"}},
	{"cutsceneMondstadt", []string{"mengde", "mdaq", "venti", "wdlq", "ambor"}},
	{"cutsceneLiyue", []string{"liyue", "lyaq", "ly_", "lq10", "lq11", "xiaopersonal", "shenhebattle", "yunjinopera"}},
	{"cutsceneInazuma", []string{"inazuma", "daoqi", "200803", "200806", "200919", "201104", "200211", "lq12", "shougunboss", "wanyexian"}},
	{"cutsceneSumeru", []string{"sumeru", "xm_", "lq13"}},
	{"cutsceneFontaine", []string{"fontaine", "lq14"}},
	{"cutsceneNatlan", []string{"natlan", "lq15"}},
	{"cutsceneNodKrai", []string{"nodkrai", "nk_", "lq16"}},
	{"cutsceneSnezhnaya", []string{"zd_", "aq70"}},
	{"cutsceneEvents", []string{"battlepass", "changeweather", "eq", "fd_", "gy", "memories", "reunion", "shieldingresources"}},
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
		if _, statErr := os.Lstat(webCaches); statErr == nil {
			return nil, err
		}
	}

	scanner := utils.FileScanner{}

	cutscenes := make(map[string]map[string]fs.DirEntry, len(cutsceneRules)+1)
	for _, r := range cutsceneRules {
		cutscenes[r.Id] = make(map[string]fs.DirEntry)
	}

	cutscenes["cutsceneOther"] = make(map[string]fs.DirEntry)

	for _, persistent := range []bool{false, true} {
		cwd := filepath.Join(streaming, "VideoAssets")
		if persistent {
			cwd = filepath.Join(g.Data, "Persistent", "VideoAssets")
		}

		for path, entry := range scanner.Scan(cwd, nil) {
			name := strings.ToLower(filepath.Base(path))

			video := strings.HasSuffix(name, ".usm") || strings.HasSuffix(name, ".usm.bak")
			if !video && !persistent {
				continue
			}

			id := "cutsceneOther"

			if video {
				for _, r := range cutsceneRules {
					if slices.ContainsFunc(r.Keywords, func(kw string) bool { return strings.Contains(name, kw) }) {
						id = r.Id

						break
					}
				}
			}

			cutscenes[id][path] = entry
		}
	}

	logs := scanner.Scan(g.Data, func(rel string) bool {
		rel = strings.ToLower(rel)

		return rel == "persistent/downloaderror.log" || strings.HasSuffix(rel, ".tmp") || strings.HasSuffix(rel, ".bak")
	})
	maps.Copy(logs, scanner.Scan(g.Root, func(rel string) bool {
		rel = strings.ToLower(rel)

		return strings.HasSuffix(rel, ".log") && !strings.Contains(rel, "/")
	}))

	ugc := scanner.Scan(filepath.Join(streaming, "AudioAssets", "BeyondUGC"), nil)
	maps.Copy(ugc, scanner.Scan(filepath.Join(g.Data, "Persistent", "AudioAssets", "BeyondUGC"), nil))

	rules := make([]ScanRule, 0, len(cutsceneRules)+5)
	for _, r := range cutsceneRules {
		rules = append(rules, ScanRule{Id: r.Id, Targets: cutscenes[r.Id]})
	}

	rules = append(rules,
		ScanRule{Id: "beyondUgc", Targets: ugc},
		ScanRule{Id: "musicGame", Targets: scanner.Scan(filepath.Join(streaming, "AudioAssets", "MusicGame"), nil)},
		ScanRule{Id: "logsJunk", Targets: logs},
	)

	var versions []*version.Version

	for _, entry := range entries {
		if entry.IsDir() {
			if ver, err := version.NewVersion(entry.Name()); err == nil {
				versions = append(versions, ver)
			}
		}
	}

	slices.SortFunc(versions, (*version.Version).Compare)

	if len(versions) > 0 {
		oldCaches := make(map[string]fs.DirEntry)
		for _, cache := range versions[:len(versions)-1] {
			maps.Copy(oldCaches, scanner.Scan(filepath.Join(webCaches, cache.String()), nil))
		}

		rules = append(rules, ScanRule{Id: "oldWebCaches", Params: []string{versions[len(versions)-1].String()}, Targets: oldCaches})
	}

	if editor {
		rules = append(rules, ScanRule{
			Id:      "beyondEditor",
			Targets: scanner.Scan(filepath.Join(g.Root, "BeyondAssets", "BeyondAssistEditor"), nil),
		})
	}

	return slices.DeleteFunc(rules, func(r ScanRule) bool { return len(r.Targets) == 0 }), scanner.Err
}
