//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Sn0wo2/genshin-cleaner/genshin"
	"github.com/Sn0wo2/genshin-cleaner/stdjson"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/pflag"
	"golang.org/x/sys/windows/registry"
)

type output struct {
	Mode         string             `json:"mode,omitempty"`
	DryRun       bool               `json:"dryRun,omitempty"`
	Game         *genshin.Genshin   `json:"game,omitempty"`
	Games        []genshin.Genshin  `json:"games,omitempty"`
	TotalItems   int                `json:"totalItems,omitempty"`
	TotalBytes   int64              `json:"totalBytes,omitempty"`
	Rules        []genshin.ScanRule `json:"rules,omitempty"`
	DeletedItems int                `json:"deletedItems,omitempty"`
	FreedBytes   int64              `json:"freedBytes,omitempty"`
	Failed       []string           `json:"failed,omitempty"`
}

func main() {
	if runtime.GOOS != "windows" {
		stdjson.New(stdjson.StageError, "only Windows is supported").Write()
		os.Exit(1)
	}

	var deleteMode, editor, dryRun, debug bool
	pflag.BoolVar(&deleteMode, "delete", false, "actually delete files")
	pflag.BoolVar(&dryRun, "dry-run", false, "preview only (overrides --delete)")
	pflag.BoolVar(&editor, "editor", false, "also remove BeyondAssistEditor (UGC editor)")
	pflag.BoolVar(&debug, "debug", false, "pretty-print (indented) JSON output")
	pflag.Parse()
	stdjson.Pretty = debug
	if dryRun {
		deleteMode = false
	}

	args := pflag.Args()
	if len(args) > 1 {
		stdjson.New(stdjson.StageError, "too many arguments, pass one game dir").Write()
		os.Exit(1)
	}

	var path string

	if len(args) > 0 {
		path = args[0]
	}

	var games []genshin.Genshin

	if path != "" {
		stdjson.New(stdjson.StageInfo, "scanning genshin dir: "+path).Write()
		var err error
		games, err = genshin.DiscoverGenshins(path)
		if err != nil {
			stdjson.New(stdjson.StageError, err.Error()).Write()
			os.Exit(1)
		}
	} else {
		stdjson.New(stdjson.StageInfo, "scanning registry genshin").Write()

		// Inspired by the registry locations used by BetterGI and Starward
		// https://github.com/babalae/better-genshin-impact/blob/b3e46b3004b8e4a1065846243a3a2a518b9a214d/BetterGenshinImpact/Genshin/Paths/RegistryGameLocator.cs#L18
		for _, subkey := range []string{
			`Software\miHoYo\HYP\1_1\hk4e_cn`,
			`Software\Cognosphere\HYP\1_0\hk4e_global`,
			`Software\miHoYo\HYP\standalone\14_0\hk4e_cn\umfgRO5gh5\hk4e_cn`,
		} {
			k, err := registry.OpenKey(registry.CURRENT_USER, subkey, registry.QUERY_VALUE)
			if err != nil {
				continue
			}

			value, _, err := k.GetStringValue("GameInstallPath")
			k.Close()
			if err != nil || value == "" {
				continue
			}

			if g, err := genshin.InspectGenshin(value); err == nil {
				games = append(games, g)
			} else {
				stdjson.New(stdjson.StageWarn, err.Error()).Write()
				continue
			}
		}

		stdjson.New(stdjson.StageInfo, fmt.Sprintf("scaned %d games", len(games))).WithData(games).Write()

		stdjson.New(stdjson.StageInfo, "scanning「LocalLow」genshin logs").Write()

		home, _ := os.UserHomeDir()

		miHoYoDir := filepath.Join(home, "AppData", "LocalLow", "miHoYo")

		seen := make(map[string]struct{})
		logPathRe := regexp.MustCompile(`(?:[A-Za-z]:[\\/]|/)[^")]*?_Data[\\/]`)
		for _, entry := range []string{
			"原神",
			"Genshin Impact",
		} {
			entryDir, err := os.Stat(filepath.Join(miHoYoDir, entry))
			if err != nil || !entryDir.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(miHoYoDir, entryDir.Name(), "output_log.txt"))
			if err != nil {
				continue
			}
			for _, match := range logPathRe.FindAllString(string(data), -1) {
				root := filepath.Dir(strings.TrimRight(match, `\/`))
				if _, ok := seen[root]; ok {
					continue
				}
				seen[root] = struct{}{}
				if g, err := genshin.InspectGenshin(root); err == nil {
					games = append(games, g)
				} else {
					stdjson.New(stdjson.StageWarn, err.Error()).Write()
					continue
				}
			}
		}
		stdjson.New(stdjson.StageInfo, fmt.Sprintf("scaned %d games", len(games))).WithData(games).Write()
		if len(games) == 0 {
			stdjson.New(stdjson.StageWarn, "failed to find genshin, scanning all drives").Write()

			var roots []string

			// 拿到所有存在的驱动器(A-Z盘)
			for letter := 'A'; letter <= 'Z'; letter++ {
				root := string(letter) + `:\`
				if _, err := os.Stat(root); err == nil {
					roots = append(roots, root)
				}
			}

			for _, root := range roots {
				var err error
				games, err = genshin.DiscoverGenshins(root)
				if err != nil {
					stdjson.New(stdjson.StageError, err.Error()).Write()
					os.Exit(1)
				}
				games = append(games, games...)
			}

			stdjson.New(stdjson.StageInfo, fmt.Sprintf("scaned %d games", len(games))).WithData(games).Write()
		}
	}

	seen := make(map[string]struct{})
	result := games[:0]
	for _, g := range games {
		key := strings.ToLower(g.Root)

		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, g)
		}
	}
	games = result

	if path == "" && len(games) > 1 {
		if deleteMode {
			stdjson.New(stdjson.StageError, "no game dir given, pass one game dir").WithData(games).Write()
			os.Exit(1)
		}
		stdjson.New(stdjson.StageResult, "list detected Genshin installs").WithData(output{Mode: "list", Games: games}).Write()
		return
	}
	if len(games) == 0 {
		stdjson.New(stdjson.StageError, "Genshin not found, pass the game dir (or its parent)").Write()
		os.Exit(1)
	}
	g := games[0]
	if len(games) > 1 {
		stdjson.New(stdjson.StageError, "multiple Genshin installs found, pass one game dir").WithData(games).Write()
		os.Exit(1)
	}

	rules, err := genshin.CollectRules(g, editor)
	if err != nil {
		stdjson.New(stdjson.StageError, "failed to scan cleanup targets").WithData(output{
			Game:   &g,
			Failed: []string{err.Error()},
		}).Write()
		os.Exit(1)
	}
	count := 0
	var total int64
	for _, r := range rules {
		count += len(r.Targets)
		for _, t := range r.Targets {
			info, err := t.Info()
			if err != nil {
				continue
			}

			total += info.Size()
		}
	}

	if !deleteMode {
		stdjson.New(stdjson.StageResult, "dry run: show files that would be deleted").WithData(output{
			Mode:       "dry-run",
			DryRun:     true,
			Game:       &g,
			TotalItems: count,
			TotalBytes: total,
			Rules:      rules,
		}).Write()
		return
	}

	running := false
	processes, err := process.Processes()
	if err != nil {
		stdjson.New(stdjson.StageError, "checking processes: "+err.Error()).Write()
		os.Exit(1)
	}
	for _, proc := range processes {
		name, _ := proc.Name()
		name = strings.ReplaceAll(strings.ToLower(name), " ", "")
		if strings.Contains(name, "yuanshen") || strings.Contains(name, "genshinimpact") {
			running = true
			break
		}
	}
	if running {
		stdjson.New(stdjson.StageError, "game is running. Close it before deleting").Write()
		os.Exit(1)
	}

	var freed int64
	var deleted int
	var failed []string
	for _, r := range rules {
		for _, t := range r.Targets {
			info, err := t.Info()
			if err != nil {
				continue
			}

			if err := os.RemoveAll(t.Name()); err != nil {
				failed = append(failed, t.Name())
			} else {
				freed += info.Size()
				deleted++
			}
		}
	}
	stdjson.New(stdjson.StageResult, "delete files").WithData(output{Mode: "delete", Game: &g, TotalItems: count, TotalBytes: total, DeletedItems: deleted, FreedBytes: freed, Failed: failed}).Write()
}
