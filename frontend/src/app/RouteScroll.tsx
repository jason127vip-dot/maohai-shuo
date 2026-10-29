import { useLayoutEffect } from 'react'
import { useLocation } from 'react-router'

export function RouteScroll() {
  const { pathname } = useLocation()
  useLayoutEffect(() => { window.scrollTo(0, 0) }, [pathname])
  return null
}
