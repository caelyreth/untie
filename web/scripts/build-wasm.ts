/// <reference types="@types/bun" />

import { copyFile, mkdir, rename, rm, stat } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const webRoot = join(dirname(fileURLToPath(import.meta.url)), '..')
const repositoryRoot = join(webRoot, '..')
const outputDir = join(webRoot, 'static', 'untie')
const wasmDir = join(webRoot, 'wasm')
const rawWasm = join(outputDir, 'untie.raw.wasm')
const outputWasm = join(outputDir, 'untie.wasm')
const temporaryWasm = join(outputDir, 'untie.wasm.tmp')
const fontSource = join(
  repositoryRoot,
  'testdata',
  'upstream',
  'fonts',
  'NewCMMath-Book.otf',
)
const fontLicense = join(
  repositoryRoot,
  'examples',
  'mygo-simple',
  'assets',
  'GUST-FONT-LICENSE.txt',
)

await mkdir(outputDir, { recursive: true })

const goRoot = (await run(['go', 'env', 'GOROOT'], webRoot)).trim()
await run(
  [
    'go',
    'build',
    '-trimpath',
    '-ldflags',
    '-s -w -buildid=',
    '-o',
    rawWasm,
    '.',
  ],
  wasmDir,
  {
    ...process.env,
    CGO_ENABLED: '0',
    GOARCH: 'wasm',
    GOOS: 'js',
  },
)

const wasmOpt = Bun.which('wasm-opt')
if (!wasmOpt) {
  throw new Error(
    'wasm-opt is required; enter the repository Nix shell first',
  )
}
await rm(temporaryWasm, { force: true })
await run(
  [
    wasmOpt,
    '--enable-bulk-memory',
    '--enable-nontrapping-float-to-int',
    '--enable-sign-ext',
    '-Oz',
    rawWasm,
    '-o',
    temporaryWasm,
  ],
  webRoot,
)
await rename(temporaryWasm, outputWasm)
await rm(rawWasm, { force: true })
await copyFresh(
  join(goRoot, 'lib', 'wasm', 'wasm_exec.js'),
  join(outputDir, 'wasm_exec.js'),
)
await copyFresh(fontSource, join(outputDir, 'NewCMMath-Book.otf'))
await copyFresh(fontLicense, join(outputDir, 'GUST-FONT-LICENSE.txt'))

const bytes = (await stat(outputWasm)).size
console.log(`untie.wasm ${(bytes / 1024 / 1024).toFixed(2)} MiB`)

async function copyFresh(source: string, destination: string) {
  await rm(destination, { force: true })
  await copyFile(source, destination)
}

async function run(
  command: string[],
  cwd: string,
  env: Record<string, string | undefined> = process.env,
) {
  const process = Bun.spawn(command, {
    cwd,
    env,
    stdout: 'pipe',
    stderr: 'pipe',
  })
  const [stdout, stderr, exitCode] = await Promise.all([
    new Response(process.stdout).text(),
    new Response(process.stderr).text(),
    process.exited,
  ])
  if (exitCode !== 0) {
    throw new Error(`${command.join(' ')} failed\n${stderr || stdout}`)
  }
  return stdout
}
