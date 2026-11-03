import { onNavigate, type OnNavigate } from '$app/navigation'
import { onDestroy } from 'svelte'
import { prefersReducedMotion } from 'svelte/motion'

export function registerRouteTransitions() {
  let active: ViewTransition | undefined

  onNavigate((navigation) => {
    if (typeof document === 'undefined') {
      return
    }
    active?.skipTransition()
    if (!canTransition(navigation)) {
      return
    }

    document.documentElement.dataset.routeTransition =
      navigation.to?.route.id === '/' ? 'back' : 'forward'

    return startTransition(navigation)
  })

  onDestroy(() => {
    active?.skipTransition()
    if (typeof document !== 'undefined') {
      delete document.documentElement.dataset.routeTransition
    }
  })

  function canTransition(navigation: OnNavigate) {
    const start = (
      document as Document & {
        startViewTransition?: Document['startViewTransition']
      }
    ).startViewTransition
    return Boolean(
      typeof start === 'function' &&
      !prefersReducedMotion.current &&
      navigation.from?.url.pathname !== navigation.to?.url.pathname,
    )
  }

  function startTransition(navigation: OnNavigate) {
    return new Promise<void>((resolve) => {
      let transition: ViewTransition
      try {
        transition = document.startViewTransition(async () => {
          resolve()
          await navigation.complete
        })
      } catch {
        resolve()
        return
      }
      active = transition
      const finish = () => {
        if (active === transition) {
          active = undefined
          delete document.documentElement.dataset.routeTransition
        }
        resolve()
      }
      void transition.finished.then(finish, finish)
    })
  }
}
