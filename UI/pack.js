const { spawnSync } = require("node:child_process")
const { randomUUID } = require("node:crypto")
const { existsSync, mkdirSync, readFileSync, rmSync, unlinkSync, writeFileSync } = require("node:fs")
const path = require("node:path")
const { parseArgs } = require("node:util")

function main() {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      output: { type: "string" },
      version: { type: "string" },
    },
  })
  const arch = positionals[0] ?? "x64"
  if (arch !== "x64" && arch !== "ARM64") {
    console.error(`invalid arch: ${arch} (choose from 'x64', 'ARM64')`)
    process.exit(2)
  }

  const ROOT = path.resolve(__dirname, "..")
  const installer = readFileSync(path.join(ROOT, "installer.wxs"), "utf8")
  const defaultVersion = installer.match(/<Package\b[^>]*?\bVersion="([^"]+)"/)?.[1]
  if (!defaultVersion) throw new Error("Could not read the WiX package version")

  let VERSION
  let BACKEND_VERSION
  if (values.version === undefined) {
    VERSION = defaultVersion
    BACKEND_VERSION = "0.0.0"
  } else {
    const raw = values.version.replace(/^v/, "")
    const match = raw.match(/^((?:0|[1-9]\d*)(?:\.(?:0|[1-9]\d*)){2,3})(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/)
    if (!match) {
      console.error(`invalid release version: ${values.version}`)
      process.exit(2)
    }
    const parts = match[1].split(".").map(Number)
    if (parts.length === 3) parts.push(0)
    if (parts[0] > 255 || parts[1] > 255 || parts.some((part, index) => index >= 2 && part > 65535)) {
      console.error(`release version is outside the MSI version range: ${values.version}`)
      process.exit(2)
    }
    VERSION = parts.join(".")
    BACKEND_VERSION = raw
  }

  const PUBLISH = path.join(ROOT, "dist", "publish")
  const MSI = path.resolve(values.output ?? path.join(ROOT, "dist", "Installer", `genshin-cleaner-${VERSION}-${arch}.msi`))

  rmSync(PUBLISH, { recursive: true, force: true })
  run("dotnet", [
    "publish", path.join(ROOT, "UI", "genshin-cleanner-ui.csproj"),
    "-nologo", "-c", "Release",
    `-p:Version=${VERSION}`,
    `-p:Platform=${arch}`,
    `-p:RuntimeIdentifier=win-${arch.toLowerCase()}`,
    `-p:BackendVersion=${BACKEND_VERSION}`,
    "--self-contained", "true",
    `-p:PublishDir=${PUBLISH}${path.sep}`,
  ])

  if (!existsSync(path.join(PUBLISH, "genshin-cleaner.pri"))) {
    throw new Error("Published WinUI resources are missing: genshin-cleaner.pri")
  }

  mkdirSync(path.dirname(MSI), { recursive: true })
  rmSync(MSI, { force: true })
  rmSync(path.join(path.dirname(MSI), `${path.basename(MSI, path.extname(MSI))}.wixpdb`), { force: true })
  let replacements = 0
  const patched = installer.replace(/(<Package\b[^>]*?\bVersion=")[^"]+(")/, (whole, prefix, suffix) => {
    replacements += 1
    return `${prefix}${VERSION}${suffix}`
  })
  if (replacements !== 1) throw new Error("Could not set the WiX package version")
  const tempInstaller = path.join(ROOT, `.installer-${randomUUID()}.wxs`)
  writeFileSync(tempInstaller, patched, "utf8")
  try {
    run("wix", [
      "build", "-nologo", "-arch", arch.toLowerCase(), "-culture", "zh-CN",
      "-ext", "WixToolset.UI.wixext/7.0.0",
      "-d", `PUBLISH=${PUBLISH}`, "-o", MSI, tempInstaller,
    ], { cwd: ROOT })
  } finally {
    unlinkSync(tempInstaller)
  }
  console.log(MSI)
}

function run(command, args, options = {}) {
  const result = spawnSync(command, args, { stdio: "inherit", ...options })
  if (result.error) throw result.error
  if (result.status !== 0) process.exit(result.status ?? 1)
}

try {
  main()
} catch (error) {
  console.error(error.message)
  process.exitCode = 1
}
