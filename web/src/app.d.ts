import type { UntieWasmAPI } from '#lib/wasm/types'

declare global {
  interface Window {
    Go: new () => {
      importObject: WebAssembly.Imports
      run(instance: WebAssembly.Instance): Promise<void>
    }
    untieAPI?: UntieWasmAPI
  }
}

export {}
