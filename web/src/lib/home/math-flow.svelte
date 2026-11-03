<script lang="ts">
  import { onMount } from 'svelte'

  import { loadUntie } from '#lib/wasm/untie'

  interface Formula {
    source: string
    dialect: 'tex' | 'typst'
    size: number
    accent: string
  }

  const formulas: readonly Formula[] = [
    {
      source: String.raw`e^{i\pi}+1=0`,
      dialect: 'tex',
      size: 22,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`\int_0^1 x^2\,dx`,
      dialect: 'tex',
      size: 24,
      accent: 'var(--regolith)',
    },
    {
      source: String.raw`sum_(k=1)^n k^2`,
      dialect: 'typst',
      size: 21,
      accent: 'var(--unripe)',
    },
    {
      source: String.raw`\nabla_\mu\phi`,
      dialect: 'tex',
      size: 22,
      accent: 'var(--life)',
    },
    {
      source: String.raw`\mathcal{L}_m`,
      dialect: 'tex',
      size: 23,
      accent: 'var(--afterglow)',
    },
    {
      source: String.raw`a^2+b^2=c^2`,
      dialect: 'tex',
      size: 21,
      accent: 'var(--remnant)',
    },
    {
      source: String.raw`\frac{-b\pm\sqrt{b^2-4ac}}{2a}`,
      dialect: 'tex',
      size: 19,
      accent: 'var(--moonveil)',
    },
    {
      source: String.raw`sqrt(x^2+y^2)`,
      dialect: 'typst',
      size: 20,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`\lim_{x\to0}\frac{\sin x}{x}`,
      dialect: 'tex',
      size: 20,
      accent: 'var(--life)',
    },
    {
      source: String.raw`mat(1, 2; 3, 4)`,
      dialect: 'typst',
      size: 19,
      accent: 'var(--regolith)',
    },
    {
      source: String.raw`\oint_{\partial\Omega}\omega`,
      dialect: 'tex',
      size: 21,
      accent: 'var(--afterglow)',
    },
    {
      source: String.raw`\prod_{p\mid n} p`,
      dialect: 'tex',
      size: 20,
      accent: 'var(--unripe)',
    },
    {
      source: String.raw`\mathbb{R}^{n\times n}`,
      dialect: 'tex',
      size: 20,
      accent: 'var(--remnant)',
    },
    {
      source: String.raw`partial_t u = Delta u`,
      dialect: 'typst',
      size: 20,
      accent: 'var(--moonveil)',
    },
    {
      source: String.raw`\det(A-\lambda I)=0`,
      dialect: 'tex',
      size: 19,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`\sum_{\mu\nu} g^{\mu\nu}R_{\mu\nu}`,
      dialect: 'tex',
      size: 19,
      accent: 'var(--regolith)',
    },
    {
      source: String.raw`1/(1+1/(1+1/x))`,
      dialect: 'typst',
      size: 18,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`\binom{n}{k}=\frac{n!}{k!(n-k)!}`,
      dialect: 'tex',
      size: 19,
      accent: 'var(--life)',
    },
    {
      source: String.raw`root(3, x+1) + sqrt(sqrt(sqrt(x)))`,
      dialect: 'typst',
      size: 18,
      accent: 'var(--afterglow)',
    },
    {
      source: String.raw`integral_0^infinity e^(-x^2) dif x = sqrt(pi)/2`,
      dialect: 'typst',
      size: 18,
      accent: 'var(--moonveil)',
    },
    {
      source: String.raw`\oint_C \vec F\cdot d\vec r`,
      dialect: 'tex',
      size: 20,
      accent: 'var(--unripe)',
    },
    {
      source: String.raw`lim_(n -> infinity) (1+1/n)^n = e`,
      dialect: 'typst',
      size: 19,
      accent: 'var(--remnant)',
    },
    {
      source: String.raw`\begin{pmatrix}a&b\\c&d\end{pmatrix}`,
      dialect: 'tex',
      size: 20,
      accent: 'var(--regolith)',
    },
    {
      source: String.raw`f(x)=cases(x^2 & x >= 0, -x & x < 0)`,
      dialect: 'typst',
      size: 18,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`\langle\phi|\psi\rangle`,
      dialect: 'tex',
      size: 22,
      accent: 'var(--afterglow)',
    },
    {
      source: String.raw`alpha beta gamma Gamma epsilon.alt theta.alt phi Omega`,
      dialect: 'typst',
      size: 19,
      accent: 'var(--life)',
    },
    {
      source: String.raw`\hat{x}\,\tilde{y}\,\vec{v}\,\dot{a}\,\ddot{b}`,
      dialect: 'tex',
      size: 19,
      accent: 'var(--moonveil)',
    },
    {
      source: String.raw`hat(f)(xi)=integral_(-infinity)^infinity f(x)e^(-2 pi i x xi) dif x`,
      dialect: 'typst',
      size: 17,
      accent: 'var(--halo)',
    },
    {
      source: String.raw`P(A\mid B)=\frac{P(B\mid A)P(A)}{P(B)}`,
      dialect: 'tex',
      size: 18,
      accent: 'var(--regolith)',
    },
  ]

  const rows = [
    [0, 17, 8, 23, 14, 27],
    [2, 20, 6, 25, 11, 26],
    [7, 19, 3, 24, 16, 28],
    [5, 21, 10, 25, 1, 26],
    [12, 18, 4, 22, 15, 27],
    [9, 26, 13, 20, 6, 28],
    [16, 23, 2, 25, 8, 27],
  ] as const

  const durations = [92, 108, 86, 116, 96, 112, 88]
  const copies = [0, 1, 2, 3, 4, 5] as const

  let rendered = $state<readonly string[]>([])
  let ready = $state(false)

  onMount(() => {
    let disposed = false
    void loadUntie()
      .then((untie) => {
        const next = formulas.map((formula, index) => {
          const result = untie.renderSVG(
            formula.source,
            formula.dialect,
            true,
            formula.size,
          )
          if (result.error) {
            console.error(`Untie Wasm formula ${index}:`, result.error)
            return ''
          }
          return result.svg
        })
        if (!disposed) {
          rendered = next
          requestAnimationFrame(() => {
            if (!disposed) {
              ready = true
            }
          })
        }
      })
      .catch((cause) => {
        console.error('Untie Wasm:', cause)
      })
    return () => {
      disposed = true
    }
  })
</script>

<div
  class="math-wall"
  class:ready
  aria-hidden="true"
  data-nosnippet
  role="presentation"
>
  {#each rows as row, rowIndex}
    <div class="row" style:--duration={`${durations[rowIndex]}s`}>
      {#each copies as copy (copy)}
        <div class="group">
          {#each row as formulaIndex}
            <div
              class="brick"
              style:--accent={formulas[formulaIndex].accent}
            >
              {#if ready && rendered[formulaIndex]}
                {@html rendered[formulaIndex]}
              {/if}
            </div>
          {/each}
        </div>
      {/each}
    </div>
  {/each}
</div>

<style>
  .math-wall {
    position: absolute;
    inset: -2px;
    z-index: 0;
    display: grid;
    grid-template-rows: repeat(7, minmax(0, 1fr));
    --brick-gap: 9px;
    gap: var(--brick-gap);
    overflow: hidden;
    opacity: 0;
    pointer-events: none;
    mask-image: linear-gradient(
      to bottom,
      transparent 0,
      #000 8%,
      #000 92%,
      transparent 100%
    );
    user-select: none;
    transition: opacity 1.1s cubic-bezier(0.16, 1, 0.3, 1);
  }

  .math-wall.ready {
    opacity: 0.3;
  }

  .row {
    display: flex;
    width: max-content;
    height: 100%;
    animation: wall-flow var(--duration) linear infinite;
    animation-play-state: paused;
  }

  .math-wall.ready .row {
    animation-play-state: running;
  }

  .row:nth-child(even) {
    margin-left: 82px;
  }

  .group {
    display: flex;
    flex: none;
    gap: var(--brick-gap);
    padding-right: var(--brick-gap);
  }

  .brick {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    flex: none;
    width: fit-content;
    min-width: 72px;
    height: 100%;
    padding: 10px 14px;
    border: 1px solid rgb(84 84 80 / 58%);
    overflow: hidden;
    color: var(--accent);
    background: rgb(26 26 26 / 52%);
  }

  .brick :global(svg) {
    display: block;
    flex: none;
    width: auto;
    height: 68%;
    max-height: 64px;
    overflow: visible;
  }

  @keyframes wall-flow {
    to {
      transform: translateX(-16.6667%);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .row {
      animation: none;
    }
  }

  @media (max-width: 700px) {
    .math-wall {
      grid-template-rows: repeat(8, minmax(0, 1fr));
      mask-image: linear-gradient(
        to bottom,
        transparent 0,
        #000 7%,
        #000 93%,
        transparent 100%
      );
    }

    .brick {
      min-width: 60px;
      padding: 8px 12px;
    }
  }
</style>
