import type { UntieWasmAPI } from '#lib/wasm/types'

declare global {
  interface ImportMetaEnv {
    readonly PUBLIC_SITE_URL?: string
  }

  interface ImportMeta {
    readonly env: ImportMetaEnv
  }

  interface Window {
    Go: new () => {
      importObject: WebAssembly.Imports
      run(instance: WebAssembly.Instance): Promise<void>
    }
    untieAPI?: UntieWasmAPI
  }
}

export {}
