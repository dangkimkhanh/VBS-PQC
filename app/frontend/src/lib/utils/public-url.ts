import { NextRequest } from 'next/server'

// Absolute URL on the address the visitor used. Behind nginx, Next's standalone
// server sees itself as localhost:3000, so req.nextUrl would send the browser
// there; nginx forwards the real host and scheme in these headers.
export const publicUrl = (path: string, req: NextRequest): URL => {
  const host = req.headers.get('x-forwarded-host') ?? req.headers.get('host')
  const proto = req.headers.get('x-forwarded-proto')?.split(',')[0].trim()
  if (!host) return new URL(path, req.nextUrl)
  return new URL(path, `${proto || req.nextUrl.protocol.replace(':', '')}://${host}`)
}
