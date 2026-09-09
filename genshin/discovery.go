package genshin

import (
	"cmp"
	"errors"
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

func DiscoverGenshins(root string) ([]Genshin, error) {
	var games []Genshin
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || path == root {
			return nil
		}

		if _, ok := skip[strings.ToLower(d.Name())]; ok { // O(1) 跳跳跳
			return filepath.SkipDir
		}

		if strings.Count(strings.TrimPrefix(path, root), string(os.PathSeparator)) > 20 { // 顶多 20 深度吧, 不然太慢了
			return filepath.SkipDir
		}

		if d.Name() != "YuanShen_Data" && d.Name() != "GenshinImpact_Data" { // YuanShen_Data | GenshinImpact_Data
			return nil
		}
		if info, err := os.Stat(filepath.Join(path, "StreamingAssets")); err == nil && info.IsDir() {
			if g, err := InspectGenshin(filepath.Dir(path)); err == nil {
				games = append(games, g)
			} else {
				return err
			}
		}
		return nil
	})
	return games, err
}

func InspectGenshin(root string) (Genshin, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return Genshin{}, err
	}

	config := make(map[string]string)

	if cfg, err := ini.Load(filepath.Join(absolute, "config.ini")); err == nil {
		for _, section := range cfg.Sections() {
			for _, key := range section.Keys() {
				config[key.Name()] = strings.Trim(strings.TrimSpace(key.String()), `"`)
			}
		}
	}

	if !strings.HasPrefix(config["game_biz"], "hk4e") { // hk4e_cn | hk4e_global | hk4e_bilibili
		return Genshin{}, errors.New("not hk4e (Genshin)")
	}

	entries, err := os.ReadDir(absolute)
	if err != nil {
		return Genshin{}, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() != "YuanShen_Data" && entry.Name() != "Genshin_Data" { // YuanShen_Data | GenshinImpact_Data
			continue
		}

		data := filepath.Join(absolute, entry.Name())
		if info, err := os.Stat(filepath.Join(data, "StreamingAssets")); err != nil || !info.IsDir() {
			continue
		}

		// game_biz前面就判空了, 所以不需要cmp.Or
		return Genshin{Root: absolute, Data: data, Biz: config["game_biz"], Ver: cmp.Or(config["game_version"], "%UNKNOWN%")}, nil
	}
	return Genshin{}, errors.New("not found Genshin")
}
