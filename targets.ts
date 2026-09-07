import fg from 'fast-glob'
import { compareVersions } from 'compare-versions'
import { readdir } from 'node:fs/promises'
import { join } from 'node:path'
import type { Game } from './discovery'

type Target = { path: string; size: number }
type CleanupRule = { title: string; note: string; targets: Target[] }

export async function collectRules(game: Game, includeEditor = false): Promise<CleanupRule[]> {
  const streamingAssets = join(game.data, 'StreamingAssets')
  const webCaches = join(game.data, 'webCaches')
  const versions = (await readdir(webCaches).catch(() => []))
    .filter(version => /^\d+(\.\d+)+$/.test(version))
    .sort(compareVersions)

  const rules: CleanupRule[] = [
    {
      title: 'Cutscene videos (*.usm)',
      note: 'launcher re-downloads on base-resource updates',
      targets: await scanFiles(join(streamingAssets, 'VideoAssets'), '**/*.usm'),
    },
    {
      title: 'BeyondUGC audio',
      note: 'launcher re-downloads on base-resource updates',
      targets: await scanFiles(join(streamingAssets, 'AudioAssets', 'BeyondUGC')),
    },
    {
      title: 'MusicGame audio',
      note: 'launcher re-downloads on base-resource updates',
      targets: await scanFiles(join(streamingAssets, 'AudioAssets', 'MusicGame')),
    },
    {
      title: 'Persistent on-demand CGs',
      note: 'downloaded on demand, stays deleted',
      targets: await scanFiles(join(game.data, 'Persistent', 'VideoAssets')),
    },
    {
      title: 'Old webCaches versions',
      note: `keeping ${versions.at(-1) ?? 'none'}`,
      targets: await Promise.all(versions.slice(0, -1).map(async version => {
        const directory = join(webCaches, version)
        return { path: directory, size: totalSize(await scanFiles(directory)) }
      })),
    },
    {
      title: 'Logs & temp junk',
      note: 'DownloadError.log, *.log, *.tmp, *.bak',
      targets: [
        ...await scanFiles(game.data, ['Persistent/DownloadError.log', 'Persistent/DownloadError.log.bak', '**/*.{tmp,bak}']),
        ...await scanFiles(game.root, '*.log'),
      ],
    },
  ]
  if (includeEditor) {
    rules.push({
      title: 'BeyondAssistEditor (UGC editor)',
      note: 'restorable via beyond_pkg_version',
      targets: await scanFiles(join(game.root, 'BeyondAssets', 'BeyondAssistEditor')),
    })
  }
  return rules
}

export function totalSize(targets: { size: number }[]): number {
  return targets.reduce((total, target) => total + target.size, 0)
}

async function scanFiles(cwd: string, patterns: string | string[] = '**'): Promise<Target[]> {
  const files = await fg(patterns, { cwd, stats: true, absolute: true, dot: true, suppressErrors: true })
  return files.map(file => ({ path: file.path, size: file.stats!.size }))
}
