import { NextRequest, NextResponse } from 'next/server'
import { publicUrl } from '@/lib/utils/public-url'

// Clears the session cookie (httpOnly, so only the server can) and returns to sign-in.
export async function GET(req: NextRequest) {
  const target = publicUrl('/auth/sign-in', req)
  if (req.nextUrl.searchParams.get('expired')) target.searchParams.set('expired', '1')
  const response = NextResponse.redirect(target)
  response.cookies.delete('session')
  return response
}
