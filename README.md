# genshin-cleaner

> Cleans up removable Genshin Impact resources(~43GB with `CNRELWin7.0.0_R47805902_S47829085_D47985349`) without triggering a full integrity check

---

> [!TIP]
> The launcher may restore some files after an update. If that happens, run `genshin-cleaner` again

## Usage

```shell
genshin-cleaner                          # dry-run if one install found, else list installs
genshin-cleaner "D:\Games\Genshin Impact"                # dry-run: JSON with dryRun flag + rules
genshin-cleaner "D:\Games\Genshin Impact" --delete       # actually delete
genshin-cleaner "D:\Games\Genshin Impact" --delete --editor
genshin-cleaner "D:\Games\Genshin Impact" --debug             # pretty-print (indented) JSON output

genshin-cleaner --help
```

## Exit codes

- `0`: finished with no issues
- `1`: failed (bad args, nothing found, game running, incomplete target scan in delete mode, ...)
- `2`: finished but incomplete: some targets could not be scanned/read/deleted. The result JSON's `failed` array lists one `path: reason` entry per problem
