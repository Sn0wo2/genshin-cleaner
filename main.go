//go:build windows

package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/3JoB/unsafeConvert"
	"github.com/Sn0wo2/genshin-cleaner/genshin"
	"github.com/Sn0wo2/genshin-cleaner/stdjson"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/pflag"
	"golang.org/x/sys/windows/registry"
)

func main() {
	var opts struct {
		list          bool
		delete        bool
		stub          bool
		unstub        bool
		stubs         bool
		editor        bool
		debug         bool
		stdin         bool
		selectionFile string
		protocol      int
	}
	pflag.BoolVar(&opts.list, "list", false, "list detected installs and exit")
	pflag.BoolVar(&opts.delete, "delete", false, "actually delete (default: dry run)")
	pflag.BoolVar(&opts.stub, "stub", false, "stub+lock instead of delete: truncate to 0 bytes and deny write/delete")
	pflag.BoolVar(&opts.unstub, "unstub", false, "remove stub locks (unlock), letting the launcher re-download")
	pflag.BoolVar(&opts.stubs, "stubs", false, "list stubbed (0-byte locked) cleanup targets and exit")
	pflag.BoolVar(&opts.editor, "editor", false, "also clean BeyondAssistEditor (UGC editor)")
	pflag.BoolVar(&opts.stdin, "stdin", false, `read selection JSON {"keep":[],"drop":[]} from stdin`)
	pflag.StringVar(&opts.selectionFile, "selection-file", "", `read selection JSON {"keep":[],"drop":[]} from file (used to hand state to an elevated relaunch)`)
	pflag.BoolVar(&opts.debug, "debug", false, "pretty-print output and emit debug logs")
	pflag.IntVar(&opts.protocol, "protocol", stdjson.Protocol, "caller's max protocol version")
	pflag.Parse()
	stdjson.Init(opts.debug)

	var dir string

	if args := pflag.Args(); len(args) > 1 {
		stdjson.Result(stdjson.StatusResultFailed, "too many arguments, pass one game dir", stdjson.NoData())
		os.Exit(1)
	} else if len(args) == 1 {
		dir = args[0]
	}

	if opts.protocol < 1 {
		stdjson.Result(stdjson.StatusResultFailed, "unsupported protocol version", stdjson.NoData())
		os.Exit(1)
	}

	var actions int

	for _, on := range []bool{opts.list, opts.delete, opts.stub, opts.unstub, opts.stubs} {
		if on {
			actions++
		}
	}

	if actions > 1 {
		stdjson.Result(stdjson.StatusResultFailed, "--list, --delete, --stub and --unstub are mutually exclusive", stdjson.NoData())
		os.Exit(1)
	}

	var sel genshin.Selection

	switch {
	case opts.selectionFile != "":
		data, err := os.ReadFile(opts.selectionFile)
		if err != nil {
			stdjson.Result(stdjson.StatusResultFailed, "read selection: "+err.Error(), stdjson.NoData())
			os.Exit(1)
		}

		if len(strings.TrimSpace(unsafeConvert.StringSlice(data))) > 0 {
			if err := json.Unmarshal(data, &sel); err != nil {
				stdjson.Result(stdjson.StatusResultFailed, "read selection: "+err.Error(), stdjson.NoData())
				os.Exit(1)
			}
		}
	case opts.stdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			stdjson.Result(stdjson.StatusResultFailed, "read selection: "+err.Error(), stdjson.NoData())
			os.Exit(1)
		}

		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &sel); err != nil {
				stdjson.Result(stdjson.StatusResultFailed, "read selection: "+err.Error(), stdjson.NoData())
				os.Exit(1)
			}
		}
	}

	var (
		games  []genshin.Genshin
		failed error
	)

	if dir != "" {
		stdjson.Log(stdjson.StatusLogInfo, "scanning genshin dir: "+dir, stdjson.StageData(stdjson.StageScan))

		var err error

		games, err = genshin.DiscoverGenshins(dir)
		if err != nil {
			if len(games) == 0 {
				stdjson.Result(stdjson.StatusResultFailed, err.Error(), stdjson.NoData())
				os.Exit(1)
			}

			failed = err
			stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())
		}
	} else {
		stdjson.Log(stdjson.StatusLogInfo, "scanning registry for genshin installs", stdjson.StageData(stdjson.StageScan))

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
				failed = errors.Join(failed, err)
				stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())
			}
		}

		stdjson.Log(stdjson.StatusLogInfo, fmt.Sprintf("scanned %d genshin installs", len(games)),
			stdjson.NewListData(games, failed))

		if len(games) == 0 {
			stdjson.Log(stdjson.StatusLogInfo, "scanning genshin logs in LocalLow", stdjson.StageData(stdjson.StageScan))

			home, _ := os.UserHomeDir()
			miHoYoDir := filepath.Join(home, "AppData", "LocalLow", "miHoYo")
			logSeen := map[string]struct{}{}
			logPathRe := regexp.MustCompile(`(?:[A-Za-z]:[\\/]|/)[^")]*?_Data[\\/]`)

			for _, entry := range []string{"原神", "Genshin Impact"} {
				entryDir, err := os.Stat(filepath.Join(miHoYoDir, entry))
				if err != nil || !entryDir.IsDir() {
					continue
				}

				data, err := os.ReadFile(filepath.Join(miHoYoDir, entryDir.Name(), "output_log.txt"))
				if err != nil {
					failed = errors.Join(failed, err)
					stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())

					continue
				}

				for _, match := range logPathRe.FindAllString(unsafeConvert.StringSlice(data), -1) {
					root := filepath.Dir(strings.TrimRight(match, `\/`))
					if _, ok := logSeen[root]; ok {
						continue
					}

					logSeen[root] = struct{}{}

					if g, err := genshin.InspectGenshin(root); err == nil {
						games = append(games, g)
					} else {
						failed = errors.Join(failed, err)
						stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())
					}
				}
			}

			if len(games) == 0 {
				stdjson.Log(stdjson.StatusLogWarn, "no genshin found, scanning all drives", stdjson.StageData(stdjson.StageScan))

				for letter := 'A'; letter <= 'Z'; letter++ {
					root := string(letter) + `:\`
					if _, err := os.Stat(root); err != nil {
						continue
					}

					found, err := genshin.DiscoverGenshins(root)
					if err != nil {
						failed = errors.Join(failed, err)
						stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())
					}

					games = append(games, found...)
				}
			}
		}
	}

	seen := map[string]struct{}{}
	games = slices.DeleteFunc(slices.Clone(games), func(g genshin.Genshin) bool {
		key := strings.ToLower(g.Root)
		if _, ok := seen[key]; ok {
			return true
		}

		seen[key] = struct{}{}

		return false
	})

	if opts.list {
		status := stdjson.StatusResultOK
		if failed != nil {
			status = stdjson.StatusResultPartial
		}

		stdjson.Result(status, "listed detected genshin installs", stdjson.NewListData(games, failed))

		if failed != nil {
			os.Exit(2)
		}

		return
	}

	if len(games) == 0 {
		stdjson.Result(stdjson.StatusResultFailed, "genshin not found, pass the game dir (or its parent)", stdjson.NoData())
		os.Exit(1)
	}

	if len(games) > 1 {
		stdjson.Result(stdjson.StatusResultFailed, "multiple genshin installs found, pass one game dir",
			stdjson.NewListData(games, failed))
		os.Exit(1)
	}

	g := games[0]

	rules, err := genshin.CollectRules(g, opts.editor)
	if err != nil && (opts.delete || rules == nil) {
		stdjson.Result(stdjson.StatusResultFailed, "failed to scan cleanup targets: "+err.Error(), stdjson.NoData())
		os.Exit(1)
	}

	rules = sel.Apply(rules)

	if err != nil {
		failed = errors.Join(failed, err)
		stdjson.Log(stdjson.StatusLogWarn, err.Error(), stdjson.NoData())
	}

	var total int64

	rulesOut := make([]stdjson.Rule, 0, len(rules))
	count := 0

	for _, r := range rules {
		targets := make([]stdjson.Target, 0, len(r.Targets))
		for p, t := range r.Targets {
			target := stdjson.Target{Path: p}

			if info, err := t.Info(); err == nil {
				size := info.Size()

				target.SizeBytes = &size
				if opts.stubs == (size != 0) {
					continue
				}

				total += size
			} else {
				if opts.stubs {
					continue
				}

				failed = errors.Join(failed, &stdjson.PathError{Path: p, Reason: err.Error()})
			}

			targets = append(targets, target)
		}

		count += len(targets)
		slices.SortFunc(targets, func(a, b stdjson.Target) int {
			sa, sb := int64(0), int64(0)
			if a.SizeBytes != nil {
				sa = *a.SizeBytes
			}

			if b.SizeBytes != nil {
				sb = *b.SizeBytes
			}

			if sa != sb {
				return cmp.Compare(sb, sa)
			}

			return strings.Compare(a.Path, b.Path)
		})
		rulesOut = append(rulesOut, stdjson.Rule{Id: r.Id, Params: r.Params, Targets: targets})
	}

	slices.SortFunc(rulesOut, func(a, b stdjson.Rule) int {
		size := func(r stdjson.Rule) (n int64) {
			for _, t := range r.Targets {
				if t.SizeBytes != nil {
					n += *t.SizeBytes
				}
			}

			return n
		}
		if sa, sb := size(a), size(b); sa != sb {
			return cmp.Compare(sb, sa)
		}

		return strings.Compare(a.Id, b.Id)
	})

	status := stdjson.StatusResultOK
	if failed != nil {
		status = stdjson.StatusResultPartial
	}

	if !opts.delete && !opts.stub && !opts.unstub {
		stage, msg := stdjson.StageDryRun, "dry run: files that would be cleaned"
		if opts.stubs {
			stage, msg = stdjson.StageStubScan, "stub scan: locked stub files"
		}

		stdjson.Result(status, msg,
			stdjson.NewDryRunData(g, stage, count, total, rulesOut, failed))

		if failed != nil {
			os.Exit(2)
		}

		return
	}

	processes, err := process.Processes()
	if err != nil {
		stdjson.Result(stdjson.StatusResultFailed, "failed to check running processes: "+err.Error(), stdjson.NoData())
		os.Exit(1)
	}

	for _, proc := range processes {
		name, _ := proc.Name()

		name = strings.ReplaceAll(strings.ToLower(name), " ", "")
		if strings.Contains(name, "yuanshen") || strings.Contains(name, "genshinimpact") {
			stdjson.Result(stdjson.StatusResultFailed, "genshin is running, close it before cleaning", stdjson.NoData())
			os.Exit(1)
		}
	}

	var freed int64

	done := 0

	for _, r := range rulesOut {
		for _, t := range r.Targets {
			if t.SizeBytes == nil {
				continue
			}

			var err error

			switch {
			case opts.stub:
				if err = genshin.Stub(t.Path); err == nil {
					freed += *t.SizeBytes
				}
			case opts.unstub:
				err = genshin.Unstub(t.Path)
			default:
				if err = os.RemoveAll(t.Path); err == nil {
					freed += *t.SizeBytes
				}
			}

			if err != nil {
				failed = errors.Join(failed, &stdjson.PathError{Path: t.Path, Reason: err.Error()})
			} else {
				done++
			}
		}
	}

	if failed != nil {
		status = stdjson.StatusResultPartial
	}

	stage, msg := stdjson.StageDelete, "deletion finished"
	if opts.stub {
		stage, msg = stdjson.StageStub, "stubbing finished"
	} else if opts.unstub {
		stage, msg = stdjson.StageUnstub, "unlocking finished"
	}

	stdjson.Result(status, msg,
		stdjson.NewDeleteData(g, stage, count, total, done, freed, failed))

	if failed != nil {
		os.Exit(2)
	}
}
