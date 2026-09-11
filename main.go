//go:build windows

package main

import (
	"fmt"
	"log/slog"
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

func main() {
	stdjson.Init(false)
	if runtime.GOOS != "windows" {
		stdjson.Log(slog.LevelError, "startup", "only Windows is supported", nil)
		os.Exit(1)
	}

	var deleteMode, editor, debug bool
	pflag.BoolVar(&deleteMode, "delete", false, "actually delete files")
	pflag.BoolVar(&editor, "editor", false, "also remove BeyondAssistEditor (UGC editor)")
	pflag.BoolVar(&debug, "debug", false, "pretty-print (indented) JSON output")
	pflag.Parse()
	stdjson.Init(debug)

	args := pflag.Args()
	if len(args) > 1 {
		stdjson.Log(slog.LevelError, "startup", "too many arguments, pass one game dir", nil)
		os.Exit(1)
	}

	var path string

	if len(args) > 0 {
		path = args[0]
	}

	var games []genshin.Genshin
	var failed []string

	if path != "" {
		stdjson.Log(slog.LevelInfo, "scan", "scanning genshin dir: "+path, nil)
		var err error
		games, err = genshin.DiscoverGenshins(path)
		if err != nil {
			if len(games) == 0 {
				stdjson.Log(slog.LevelError, "scan", err.Error(), nil)
				os.Exit(1)
			}
			failed = append(failed, strings.Split(err.Error(), "\n")...)
			stdjson.Log(slog.LevelWarn, "scan", err.Error(), nil)
		}
	} else {
		stdjson.Log(slog.LevelInfo, "scan", "scanning registry genshin", nil)

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
				failed = append(failed, err.Error())
				stdjson.Log(slog.LevelWarn, "scan", err.Error(), nil)
				continue
			}
		}

		stdjson.Log(slog.LevelInfo, "scan", fmt.Sprintf("scaned %d games", len(games)), games)

		stdjson.Log(slog.LevelInfo, "scan", "scanning「LocalLow」genshin logs", nil)

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
				failed = append(failed, err.Error())
				stdjson.Log(slog.LevelWarn, "scan", err.Error(), nil)
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
					failed = append(failed, err.Error())
					stdjson.Log(slog.LevelWarn, "scan", err.Error(), nil)
					continue
				}
			}
		}
		stdjson.Log(slog.LevelInfo, "scan", fmt.Sprintf("scaned %d games", len(games)), games)
		if len(games) == 0 {
			stdjson.Log(slog.LevelWarn, "scan", "failed to find genshin, scanning all drives", nil)

			var roots []string

			// 拿到所有存在的驱动器(A-Z盘)
			for letter := 'A'; letter <= 'Z'; letter++ {
				root := string(letter) + `:\`
				if _, err := os.Stat(root); err == nil {
					roots = append(roots, root)
				}
			}

			for _, root := range roots {
				discovered, err := genshin.DiscoverGenshins(root)
				if err != nil {
					failed = append(failed, strings.Split(err.Error(), "\n")...)
					stdjson.Log(slog.LevelWarn, "scan", err.Error(), nil)
				}
				games = append(games, discovered...)
			}

			stdjson.Log(slog.LevelInfo, "scan", fmt.Sprintf("scaned %d games", len(games)), games)
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
			stdjson.Log(slog.LevelError, "select", "no game dir given, pass one game dir", games)
			os.Exit(1)
		}
		stdjson.Log(slog.LevelInfo, "list", "list detected Genshin installs", struct {
			Mode   string            `json:"mode,omitempty"`
			Games  []genshin.Genshin `json:"games,omitempty"`
			Failed []string          `json:"failed,omitempty"`
		}{
			Mode:   "list",
			Games:  games,
			Failed: failed,
		})
		if len(failed) > 0 {
			os.Exit(2)
		}
		return
	}
	if len(games) == 0 {
		stdjson.Log(slog.LevelError, "select", "Genshin not found, pass the game dir (or its parent)", nil)
		os.Exit(1)
	}
	g := games[0]
	if len(games) > 1 {
		stdjson.Log(slog.LevelError, "select", "multiple Genshin installs found, pass one game dir", games)
		os.Exit(1)
	}

	rules, err := genshin.CollectRules(g, editor)
	if err != nil && (deleteMode || rules == nil) {
		stdjson.Log(slog.LevelError, "collect", "failed to scan cleanup targets: "+err.Error(), struct {
			Game   *genshin.Genshin `json:"game,omitempty"`
			Failed []string         `json:"failed,omitempty"`
		}{Game: &g, Failed: []string{err.Error()}})
		os.Exit(1)
	}
	if err != nil {
		failed = append(failed, strings.Split(err.Error(), "\n")...)
		stdjson.Log(slog.LevelWarn, "collect", err.Error(), nil)
	}
	count := 0
	var total int64
	for _, r := range rules {
		count += len(r.Targets)
		for p, t := range r.Targets {
			info, err := t.Info()
			if err != nil {
				if !deleteMode {
					failed = append(failed, fmt.Sprintf("%s: %s", p, err))
				}
				continue
			}

			total += info.Size()
		}
	}

	if !deleteMode {
		stdjson.Log(slog.LevelInfo, "dry-run", "dry run: show files that would be deleted", struct {
			Mode       string             `json:"mode,omitempty"`
			DryRun     bool               `json:"dryRun,omitempty"`
			Game       *genshin.Genshin   `json:"game,omitempty"`
			TotalItems int                `json:"totalItems,omitempty"`
			TotalBytes int64              `json:"totalBytes,omitempty"`
			Rules      []genshin.ScanRule `json:"rules,omitempty"`
			Failed     []string           `json:"failed,omitempty"`
		}{
			Mode:       "dry-run",
			DryRun:     true,
			Game:       &g,
			TotalItems: count,
			TotalBytes: total,
			Rules:      rules,
			Failed:     failed,
		})
		if len(failed) > 0 {
			os.Exit(2)
		}
		return
	}

	running := false
	processes, err := process.Processes()
	if err != nil {
		stdjson.Log(slog.LevelError, "check", "checking processes: "+err.Error(), nil)
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
		stdjson.Log(slog.LevelError, "check", "genshin is running. Close it before deleting", nil)
		os.Exit(1)
	}

	var freed int64
	var deleted int
	for _, r := range rules {
		for p, t := range r.Targets {
			info, err := t.Info()
			if err != nil {
				failed = append(failed, fmt.Sprintf("%s: %s", p, err))
				continue
			}

			if err := os.RemoveAll(p); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %s", p, err))
			} else {
				freed += info.Size()
				deleted++
			}
		}
	}
	stdjson.Log(slog.LevelInfo, "delete", "delete files", struct {
		Mode         string           `json:"mode,omitempty"`
		Game         *genshin.Genshin `json:"game,omitempty"`
		TotalItems   int              `json:"totalItems,omitempty"`
		TotalBytes   int64            `json:"totalBytes,omitempty"`
		DeletedItems int              `json:"deletedItems,omitempty"`
		FreedBytes   int64            `json:"freedBytes,omitempty"`
		Failed       []string         `json:"failed,omitempty"`
	}{
		Mode:         "delete",
		Game:         &g,
		TotalItems:   count,
		TotalBytes:   total,
		DeletedItems: deleted,
		FreedBytes:   freed,
		Failed:       failed,
	})
	if len(failed) > 0 {
		os.Exit(2)
	}
}
