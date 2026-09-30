import { NextRequest, NextResponse } from 'next/server'
import { decrypt } from '@/lib/auth/session'
import { cookies } from 'next/headers'
import { publicUrl } from '@/lib/utils/public-url'

// 1. Specify protected and public routes
const protectedRoutes = ['/education-admin', '/student', '/admin']
const adminRoutes = ['/admin']
const educationAdminRoutes = ['/education-admin']
const studentRoutes = ['/student']
// Signed-in users are sent to their dashboard from these pages. Activation and
// reset links stay reachable so they work even in a browser with another session.
const authRoutes = ['/auth/sign-in', '/auth/sign-up', '/auth/forgot-password', '/auth/university-contact']

export default async function middleware(req: NextRequest) {
  // 2. Check if the current route is protected or public
  const path = req.nextUrl.pathname
  const isProtectedRoute = protectedRoutes.some((route) => path.startsWith(route))
  const isEducationAdminRoute = educationAdminRoutes.some((route) => path.startsWith(route))
  const isStudentRoute = studentRoutes.some((route) => path.startsWith(route))
  const isAdminRoute = adminRoutes.some((route) => path.startsWith(route))
  const isPublicRoute = authRoutes.includes(path)

  if (path === '/auth/education-sign-up') {
    return NextResponse.redirect(publicUrl('/auth/university-contact', req))
  }

  //3. Decrypt the session from the cookie
  const cookie = (await cookies()).get('session')?.value
  const session = await decrypt(cookie)

  //4. Redirect to /auth/sign-in if the user is not authenticated and trying to access protected routes
  if (isProtectedRoute && !session?.access_token) {
    return NextResponse.redirect(publicUrl('/auth/sign-in', req))
  }

  // 5. Role-based access control for authenticated users
  if (session?.access_token) {
    const userRole = session.role

    // Check if education admin is trying to access student routes
    if (userRole === 'university_admin' && (isStudentRoute || isAdminRoute)) {
      return NextResponse.redirect(publicUrl('/education-admin', req))
    }

    // Check if student is trying to access admin routes
    if (userRole === 'student' && (isEducationAdminRoute || isAdminRoute)) {
      return NextResponse.redirect(publicUrl('/student', req))
    }

    // Check if admin is trying to access education admin routes
    if (userRole === 'admin' && (isEducationAdminRoute || isStudentRoute)) {
      return NextResponse.redirect(publicUrl('/admin', req))
    }

    // 6. Redirect authenticated users from public routes to their respective dashboards
    if (isPublicRoute) {
      if (userRole === 'university_admin') {
        return NextResponse.redirect(publicUrl('/education-admin', req))
      } else if (userRole === 'student') {
        return NextResponse.redirect(publicUrl('/student', req))
      } else if (userRole === 'admin') {
        return NextResponse.redirect(publicUrl('/admin', req))
      }
    }
  }

  return NextResponse.next()
}

// Routes Middleware should not run on
export const config = {
  matcher: ['/((?!api|_next/static|_next/image|.*\\.png$).*)']
}
