import type { RangiTheme } from 'comark/plugins/rangi'

export const untieRangiTheme: RangiTheme = {
  name: 'untie',
  scheme: 'dark',
  bg: 'var(--page)',
  fg: 'var(--ink)',
  numbers: 'var(--muted)',
  tokens: {
    kwd: 'var(--afterglow)',
    section: 'var(--halo)',
    class: 'var(--regolith)',
    cmnt: 'var(--muted)',
    bracket: 'var(--secondary)',
    num: 'var(--moonveil)',
    bool: 'var(--remnant)',
    str: 'var(--unripe)',
    esc: 'var(--regolith)',
    insert: 'var(--life)',
    deleted: 'var(--afterglow)',
    err: 'var(--afterglow)',
    var: 'var(--ink)',
    type: 'var(--halo)',
    func: 'var(--life)',
    oper: 'var(--secondary)',
  },
}
