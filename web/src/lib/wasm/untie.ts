import type { UntieWasmAPI } from './types'

let loadPromise: Promise<UntieWasmAPI> | undefined

export function loadUntie(): Promise<UntieWasmAPI> {
  loadPromise ??= start()
  return loadPromise
}

async function start(): Promise<UntieWasmAPI> {
  if (typeof window === 'undefined') {
    throw new Error('Untie Wasm requires a browser')
  }
  await ensureGoRuntime()
  const go = new window.Go()
  const instance = await instantiate(go)
  void go.run(instance)
  const api = await waitForAPI()
  const font = await fetch('/untie/NewCMMath-Book.otf')
  if (!font.ok) {
    throw new Error(`load math font: ${font.status}`)
  }
  const error = api.initFont(new Uint8Array(await font.arrayBuffer()))
  if (error) {
    throw new Error(error)
  }
  return wrapAPI(api)
}

async function ensureGoRuntime() {
  if (window.Go) {
    return
  }
  const runtimeURL: string = '/untie/wasm_exec.js'
  await import(/* @vite-ignore */ runtimeURL)
  if (!window.Go) {
    throw new Error('Go Wasm runtime was not loaded')
  }
}

async function instantiate(go: InstanceType<typeof window.Go>) {
  const response = await fetch('/untie/untie.wasm')
  if (!response.ok) {
    throw new Error(`load untie.wasm: ${response.status}`)
  }
  let instance: WebAssembly.Instance
  try {
    const result = await WebAssembly.instantiateStreaming(
      response,
      go.importObject,
    )
    instance = result.instance
  } catch {
    const bytes = await fetch('/untie/untie.wasm').then((value) =>
      value.arrayBuffer(),
    )
    const result = await WebAssembly.instantiate(bytes, go.importObject)
    instance = result.instance
  }
  return instance
}

function wrapAPI(api: UntieWasmAPI): UntieWasmAPI {
  return {
    initFont(data) {
      return api.initFont(data)
    },
    renderSVG(source, dialect, display) {
      const result = api.renderSVG(source, dialect, display)
      return {
        error: result.error,
        svg: result.svg.replaceAll('fill="#000000"', 'fill="currentColor"'),
      }
    },
  }
}

async function waitForAPI(): Promise<UntieWasmAPI> {
  for (let attempt = 0; attempt < 500; attempt += 1) {
    if (window.untieAPI) {
      return window.untieAPI
    }
    await new Promise((resolve) => setTimeout(resolve, 20))
  }
  throw new Error('Untie Wasm did not expose its API')
}
