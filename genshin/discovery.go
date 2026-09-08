package genshin

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/ini.v1"
)

var skip = map[string]struct{}{
	"windows":                   {},
	"$recycle.bin":              {},
	"system volume information": {},
	"recovery":                  {},
	"$sysreset":                 {},
	"msocache":                  {},
	"perflogs":                  {},
	"temp":                      {},
	"intel":                     {},
	"amd":                       {},
	"nvidia":                    {},
	"scoop":                     {},
	"chocolatey":                {},
	"windowsapps":               {},
	"programdata":               {},
	"msvc":                      {},
	"msvcshared":                {},
	"node_modules":              {},
	".git":                      {},
	"venv":                      {},
	".venv":                     {},
	"target":                    {},
}

func DiscoverGames(root string) []Game {
	var games []Game
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}

		if _, ok := skip[strings.ToLower(d.Name())]; ok { // 跳跳跳
			return filepath.SkipDir
		}

		if strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator)) > 20 { // 顶多 20 深度吧, 不然太慢了
			return filepath.SkipDir
		}

		if strings.HasSuffix(d.Name(), "_Data") { // YuanShen_Data | GenshinImpact_Data
			if info, err := os.Stat(filepath.Join(path, "StreamingAssets")); err == nil && info.IsDir() {
				if g, ok := InspectGame(filepath.Dir(path)); ok {
					games = append(games, g)
				}
			}
		}
		return nil
	})
	return games
}

func InspectGame(root string) (Game, bool) {
	config := make(map[string]string)

	if cfg, err := ini.Load(filepath.Join(root, "config.ini")); err == nil {
		for _, section := range cfg.Sections() {
			for _, key := range section.Keys() {
				config[key.Name()] = strings.Trim(strings.TrimSpace(key.String()), `"`)
			}
		}
	}

	if !strings.HasPrefix(config["game_biz"], "hk4e") { // hk4e_cn | hk4e_global | hk4e_bilibili
		return Game{}, false
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return Game{}, false
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasSuffix(entry.Name(), "_Data") { // YuanShen_Data | GenshinImpact_Data
			continue
		}

		data := filepath.Join(root, entry.Name())
		if info, err := os.Stat(filepath.Join(data, "StreamingAssets")); err != nil || !info.IsDir() {
			continue
		}
		version := config["game_version"]
		if version == "" {
			version = "?"
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return Game{}, false
		}
		return Game{Root: absolute, Data: filepath.Join(absolute, entry.Name()), Biz: config["game_biz"], Ver: version}, true
	}
	return Game{}, false
}
