import { program } from 'commander'
import fg from 'fast-glob'
import prompts from 'prompts'
import bytes from 'bytes'
import { compareVersions } from 'compare-versions'
import ini from 'ini'
import { existsSync } from 'node:fs'
import { readdir, readFile, rm } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { join, dirname } from 'node:path'
import { homedir, platform } from 'node:os'

program
  .name('clean-genshin')
  .description('Remove deletable Genshin Impact files. The launcher may restore some files after resource updates.')
  .argument('[path]', 'game directory or its parent')
  .option('--yes', 'delete without confirmation (for scripts)')
  .option('--editor', 'also remove BeyondAssistEditor (UGC editor)')
  .parse()
const o = program.opts()
const path = program.args[0]

const glob = (pattern: string | string[], cwd: string) =>
  fg(pattern, { cwd, stats: true, absolute: true, dot: true, suppressErrors: true })
const toTargets = (files: any[]) => files.map((f: any) => ({ path: f.path, size: f.stats.size }))
const dirFiles = async (dir: string) => glob('**', dir)
const totalSize = (targets: { size: number }[]) => targets.reduce((total, t) => total + t.size, 0)
type Game = { root: string; data: string; biz: string; ver: string }
const inspectGame = async (root: string, configText?: string): Promise<Game | undefined> => {
  const text = configText ?? await readFile(join(root, 'config.ini'), 'utf8').catch(() => '')
  if (!/game_biz\s*=\s*hk4e/i.test(text)) return
  const cfg = ini.parse(text)
  const s = (cfg.General ?? cfg) as any
  const biz = String(s.game_biz ?? '')
  if (!biz.startsWith('hk4e')) return
  const data = (await readdir(root, { withFileTypes: true }).catch(() => []))
    .find(e => e.isDirectory() && /_Data$/i.test(e.name) && existsSync(join(root, e.name, 'StreamingAssets')))
  return data && { root, data: join(root, data.name), biz, ver: String(s.game_version ?? '?') }
}
const discoverGames = async (cwd: string, patterns: string | string[], deep: number) => {
  const configs = await fg(patterns, {
    cwd,
    onlyFiles: true,
    absolute: true,
    dot: false,
    deep,
    followSymbolicLinks: false,
    suppressErrors: true,
  })
  const games: Game[] = []
  for (const configPath of configs) {
    const text = await readFile(configPath, 'utf8').catch(() => '')
    const game = await inspectGame(dirname(configPath), text)
    if (game) games.push(game)
  }
  return games
}
const registryPaths = (key: string) => {
  const paths = new Set<string>()
  for (const view of ['64', '32']) {
    try {
      const output = execFileSync('reg.exe', ['query', key, '/s', `/reg:${view}`], { encoding: 'utf8', windowsHide: true })
      for (const line of output.split(/\r?\n/)) {
        const match = line.match(/^\s*(?:GameInstallPath|InstallPath|InstallLocation)\s+REG_\w+\s+(.+?)\s*$/i)
        if (match) paths.add(match[1].replace(/^"|"$/g, '').trim())
      }
    } catch {}
  }
  return [...paths]
}
const discoverRegistryGames = async () => {
  if (platform() !== 'win32') return [] as Game[]
  const keys = [
    'HKCU\\SOFTWARE\\miHoYo\\HYP',
    'HKCU\\SOFTWARE\\Cognosphere\\HYP',
    'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Genshin Impact',
    'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\\u539f\u795e',
  ]
  const games: Game[] = []
  for (const launcherPath of keys.flatMap(registryPaths)) {
    const direct = await inspectGame(launcherPath)
    if (direct) { games.push(direct); continue }
    const launcherConfig = await readFile(join(launcherPath, 'config.ini'), 'utf8').catch(() => '')
    const cfg = ini.parse(launcherConfig)
    const s = (cfg.General ?? cfg) as any
    const gamePath = String(s.game_install_path ?? '').trim().replace(/^"|"$/g, '')
    if (!gamePath) continue
    const resolved = /^[A-Za-z]:[\\/]|^\\\\|^\//.test(gamePath) ? gamePath : join(launcherPath, gamePath)
    const game = await inspectGame(resolved)
    if (game) games.push(game)
  }
  return [...new Map(games.map(g => [g.root, g])).values()]
}

try {
  let games: Game[]
  if (path) {
    games = await discoverGames(path, ['config.ini', '*/config.ini'], 1)
  } else {
    games = await discoverRegistryGames()
    if (!games.length) {
      const cwds = platform() === 'win32'
        ? Array.from({ length: 26 }, (_, i) => `${String.fromCharCode(65 + i)}:\\`).filter(d => existsSync(d))
        : [homedir(), '/opt', ...(await Promise.all(['/mnt', '/run/media', '/media'].map(m => readdir(m, { withFileTypes: true }).then(es => es.filter(e => e.isDirectory()).map(e => join(m, e.name))).catch(() => [])))).flat()]
      games = [...new Map((await Promise.all(cwds.map(cwd => discoverGames(cwd, '**/config.ini', 5)))).flat().map(g => [g.root, g])).values()]
    }
  }
  if (!games.length) throw new Error('Genshin not found — pass the game dir (or its parent).')
  let game = games[0]
  if (games.length > 1) {
    if (o.yes || !process.stdin.isTTY) throw new Error('Multiple Genshin installs found — pass one game dir.')
    const pick = await prompts({ type: 'select', name: 'g', message: 'Multiple installs found:', choices: games.map(g => ({ title: `${g.root} (${g.ver} ${g.biz})`, value: g })) })
    if (!pick.g) process.exit()
    game = pick.g
  }
  console.log(`Game: ${game.root} (${game.ver}, ${game.biz})`)

  const sa = join(game.data, 'StreamingAssets')
  const wc = join(game.data, 'webCaches')
  const versions = (await readdir(wc).catch(() => [] as string[])).filter(v => /^\d+(\.\d+)+$/.test(v)).sort(compareVersions)
  const oldWc = await Promise.all(versions.slice(0, -1).map(async v => ({ path: join(wc, v), size: totalSize(toTargets(await dirFiles(join(wc, v)))) })))

  const rules: { title: string; note: string; targets: { path: string; size: number }[] }[] = [
    { title: 'Cutscene videos (*.usm)', note: 'launcher re-downloads on base-resource updates', targets: toTargets(await glob('**/*.usm', join(sa, 'VideoAssets'))) },
    { title: 'BeyondUGC audio', note: 'launcher re-downloads on base-resource updates', targets: toTargets(await dirFiles(join(sa, 'AudioAssets', 'BeyondUGC'))) },
    { title: 'MusicGame audio', note: 'launcher re-downloads on base-resource updates', targets: toTargets(await dirFiles(join(sa, 'AudioAssets', 'MusicGame'))) },
    { title: 'Persistent on-demand CGs', note: 'downloaded on demand, stays deleted', targets: toTargets(await dirFiles(join(game.data, 'Persistent', 'VideoAssets'))) },
    { title: 'Old webCaches versions', note: `keeping ${versions.at(-1) ?? 'none'}`, targets: oldWc },
    { title: 'Logs & temp junk', note: 'DownloadError.log, *.log, *.tmp, *.bak', targets: toTargets([...await glob(['Persistent/DownloadError.log', 'Persistent/DownloadError.log.bak', '**/*.{tmp,bak}'], game.data), ...await glob('*.log', game.root)]) },
  ]
  if (o.editor) rules.push({ title: 'BeyondAssistEditor (UGC editor)', note: 'restorable via beyond_pkg_version', targets: toTargets(await dirFiles(join(game.root, 'BeyondAssets', 'BeyondAssistEditor'))) })

  const active = rules.filter(r => r.targets.length)
  if (!active.length) { console.log('Nothing to clean.'); process.exit() }
  const count = active.reduce((s, r) => s + r.targets.length, 0)
  const total = active.reduce((s, r) => s + totalSize(r.targets), 0)
  console.log()
  for (const r of active) {
    console.log(`- ${r.title}: ${r.targets.length} item(s), ${bytes(totalSize(r.targets))} (${r.note})`)
    for (const t of r.targets.slice(0, 3)) console.log(`    ${t.path}`)
    if (r.targets.length > 3) console.log(`    ... ${r.targets.length - 3} more`)
  }
  console.log(`\nTotal: ${count} item(s), ${bytes(total)}`)

  if (!o.yes) {
    if (!process.stdin.isTTY) { console.log('\nPreview only — pass --yes to delete.'); process.exit() }
    const { yes } = await prompts({ type: 'confirm', name: 'yes', message: `Delete ${count} item(s), ${bytes(total)}?`, initial: false })
    if (!yes) { console.log('Canceled.'); process.exit() }
  }
  let procs = ''
  try {
    procs = platform() === 'win32'
      ? execFileSync('tasklist', ['/FO', 'CSV', '/NH'], { encoding: 'utf8' })
      : execFileSync('pgrep', ['-laf', 'yuanshen|genshinimpact'], { encoding: 'utf8' })
  } catch {}
  if (/yuanshen|genshinimpact/i.test(procs)) {
    throw new Error('Game is running. Close it before deleting.')
  }
  let freed = 0, ok = 0
  const failed: string[] = []
  for (const r of active) for (const t of r.targets) {
    try { await rm(t.path, { recursive: true, force: true }); freed += t.size; ok++ } catch { failed.push(t.path) }
  }
  console.log(`Deleted ${ok} item(s), freed ${bytes(freed)}.`)
  if (failed.length) console.log(`Failed ${failed.length}:\n  ${failed.slice(0, 5).join('\n  ')}`)
  console.log('Rerun after the next base-resource update to clean what the launcher restored.')
} catch (e: any) {
  console.error(`Error: ${e.message ?? e}`)
  process.exit(1)
}
