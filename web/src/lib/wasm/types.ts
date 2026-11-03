export interface WasmRenderResult {
  svg: string
  error: string
}

export interface UntieWasmAPI {
  initFont(data: Uint8Array): string
  renderSVG(
    source: string,
    dialect: string,
    display: boolean,
  ): WasmRenderResult
}
