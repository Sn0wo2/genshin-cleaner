import { program } from 'commander'
import prompts from 'prompts'
import bytes from 'bytes'
import { rm } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { platform } from 'node:os'
import { findGames } from './discovery'
import { collectRules, totalSize } from './targets'

program
  .name('genshin-cleaner')
  .description('Remove deletable Genshin Impact files. The launcher may restore some files after resource updates.')
  .argument('[path]', 'game directory or its parent')
  .option('--yes', 'delete without confirmation (for scripts)')
  .option('--editor', 'also remove BeyondAssistEditor (UGC editor)')
  .parse()
const options = program.opts<{ yes?: boolean; editor?: boolean }>()

try {
  const games = await findGames(program.args[0])
  if (!games.length) throw new Error('Genshin not found — pass the game dir (or its parent).')

  let game = games[0]
  if (games.length > 1) {
    if (options.yes || !process.stdin.isTTY) {
      throw new Error('Multiple Genshin installs found — pass one game dir.')
    }
    const { selectedGame } = await prompts({
      type: 'select',
      name: 'selectedGame',
      message: 'Multiple installs found:',
      choices: games.map(game => ({ title: `${game.root} (${game.ver} ${game.biz})`, value: game })),
    })
    if (!selectedGame) process.exit()
    game = selectedGame
  }
  console.log(`Game: ${game.root} (${game.ver}, ${game.biz})`)

  const activeRules = (await collectRules(game, options.editor)).filter(rule => rule.targets.length)
  if (!activeRules.length) {
    console.log('Nothing to clean.')
    process.exit()
  }
  const count = activeRules.reduce((sum, rule) => sum + rule.targets.length, 0)
  const total = activeRules.reduce((sum, rule) => sum + totalSize(rule.targets), 0)
  console.log()
  for (const rule of activeRules) {
    console.log(`- ${rule.title}: ${rule.targets.length} item(s), ${bytes(totalSize(rule.targets))} (${rule.note})`)
    for (const target of rule.targets.slice(0, 3)) console.log(`    ${target.path}`)
    if (rule.targets.length > 3) console.log(`    ... ${rule.targets.length - 3} more`)
  }
  console.log(`\nTotal: ${count} item(s), ${bytes(total)}`)

  if (!options.yes) {
    if (!process.stdin.isTTY) {
      console.log('\nPreview only — pass --yes to delete.')
      process.exit()
    }
    const { yes } = await prompts({
      type: 'confirm',
      name: 'yes',
      message: `Delete ${count} item(s), ${bytes(total)}?`,
      initial: false,
    })
    if (!yes) {
      console.log('Canceled.')
      process.exit()
    }
  }
  if (isGameRunning()) {
    throw new Error('Game is running. Close it before deleting.')
  }

  let freed = 0
  let deleted = 0
  const failed: string[] = []
  for (const rule of activeRules) {
    for (const target of rule.targets) {
      try {
        await rm(target.path, { recursive: true, force: true })
        freed += target.size
        deleted++
      } catch {
        failed.push(target.path)
      }
    }
  }
  console.log(`Deleted ${deleted} item(s), freed ${bytes(freed)}.`)
  if (failed.length) console.log(`Failed ${failed.length}:\n  ${failed.slice(0, 5).join('\n  ')}`)
  console.log('Rerun after the next base-resource update to clean what the launcher restored.')
} catch (error: any) {
  console.error(`Error: ${error.message ?? error}`)
  process.exit(1)
}

function isGameRunning(): boolean {
  try {
    const processes = platform() === 'win32'
      ? execFileSync('tasklist', ['/FO', 'CSV', '/NH'], { encoding: 'utf8' })
      : execFileSync('pgrep', ['-laf', 'yuanshen|genshinimpact'], { encoding: 'utf8' })
    return /yuanshen|genshinimpact/i.test(processes)
  } catch {
    return false
  }
}
