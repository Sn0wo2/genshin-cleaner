import fg from 'fast-glob'
import ini from 'ini'
import { existsSync } from 'node:fs'
import { readdir, readFile } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { dirname, join } from 'node:path'
import { homedir, platform } from 'node:os'

export type Game = { root: string; data: string; biz: string; ver: string }

export async function findGames(path?: string): Promise<Game[]> {
  if (path) return discoverGames(path, ['config.ini', '*/config.ini'], 1)

  let games = await discoverRegistryGames()
  if (!games.length) {
    const searchRoots: string[] = []
    if (platform() === 'win32') {
      for (let letter = 65; letter <= 90; letter++) {
        const driveRoot = `${String.fromCharCode(letter)}:\\`
        if (existsSync(driveRoot)) searchRoots.push(driveRoot)
      }
    } else {
      searchRoots.push(homedir(), '/opt')
      const mountDirectories = await Promise.all(
        ['/mnt', '/run/media', '/media'].map(async mountRoot => {
          const entries = await readdir(mountRoot, { withFileTypes: true }).catch(() => [])
          return entries.filter(entry => entry.isDirectory()).map(entry => join(mountRoot, entry.name))
        }),
      )
      searchRoots.push(...mountDirectories.flat())
    }
    games = (await Promise.all(searchRoots.map(root => discoverGames(root, '**/config.ini', 5)))).flat()
  }
  return [...new Map(games.map(game => [game.root, game])).values()]
}

async function discoverGames(cwd: string, patterns: string | string[], deep: number): Promise<Game[]> {
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
    const game = await inspectGame(dirname(configPath))
    if (game) games.push(game)
  }
  return games
}

async function discoverRegistryGames(): Promise<Game[]> {
  if (platform() !== 'win32') return []

  const keys = [
    'HKCU\\SOFTWARE\\miHoYo\\HYP',
    'HKCU\\SOFTWARE\\Cognosphere\\HYP',
    'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Genshin Impact',
    'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\\u539f\u795e',
  ]
  const games: Game[] = []
  for (const launcherPath of keys.flatMap(registryPaths)) {
    const directGame = await inspectGame(launcherPath)
    if (directGame) {
      games.push(directGame)
      continue
    }
    const config = ini.parse(await readFile(join(launcherPath, 'config.ini'), 'utf8').catch(() => ''))
    const section = config.General ?? config
    const gamePath = String(section.game_install_path ?? '').trim().replace(/^"|"$/g, '')
    if (!gamePath) continue

    const game = await inspectGame(
      /^[A-Za-z]:[\\/]|^\\\\|^\//.test(gamePath) ? gamePath : join(launcherPath, gamePath),
    )
    if (game) games.push(game)
  }
  return games
}

async function inspectGame(root: string): Promise<Game | undefined> {
  const text = await readFile(join(root, 'config.ini'), 'utf8').catch(() => '')
  if (!/game_biz\s*=\s*hk4e/i.test(text)) return

  const config = ini.parse(text)
  const section = config.General ?? config
  const business = String(section.game_biz ?? '')
  if (!business.startsWith('hk4e')) return

  const dataDirectory = (await readdir(root, { withFileTypes: true }).catch(() => []))
    .find(entry => entry.isDirectory()
      && /_Data$/i.test(entry.name)
      && existsSync(join(root, entry.name, 'StreamingAssets')))
  return dataDirectory && {
    root,
    data: join(root, dataDirectory.name),
    biz: business,
    ver: String(section.game_version ?? '?'),
  }
}

function registryPaths(key: string): string[] {
  const paths = new Set<string>()
  for (const view of ['64', '32']) {
    try {
      const output = execFileSync('reg.exe', ['query', key, '/s', `/reg:${view}`], {
        encoding: 'utf8',
        windowsHide: true,
      })
      for (const line of output.split(/\r?\n/)) {
        const match = line.match(/^\s*(?:GameInstallPath|InstallPath|InstallLocation)\s+REG_\w+\s+(.+?)\s*$/i)
        if (match) paths.add(match[1].replace(/^"|"$/g, '').trim())
      }
    } catch {}
  }
  return [...paths]
}
