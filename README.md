# genshin-cleaner

> Cleans up removable Genshin Impact resources(~43GB with `CNRELWin7.0.0_R47805902_S47829085_D47985349`) without triggering a full integrity check

---

> [!TIP]
> The launcher may restore some files after an update. If that happens, run `genshin-cleaner` again

## Usage

```shell
genshin-cleaner                          # dry-run if one install found, else list installs
genshin-cleaner --dry-run                # dry-run without a path (if one install is found)
genshin-cleaner "D:\Games\Genshin Impact"                # dry-run: JSON with dryRun flag + rules
genshin-cleaner "D:\Games\Genshin Impact" --dry-run      # same as above, explicit
genshin-cleaner "D:\Games\Genshin Impact" --delete       # actually delete
genshin-cleaner "D:\Games\Genshin Impact" --delete --editor
genshin-cleaner "D:\Games\Genshin Impact" --delete --dry-run  # safety: dry-run, never delete
genshin-cleaner "D:\Games\Genshin Impact" --debug             # pretty-print (indented) JSON output

genshin-cleaner --help
```
