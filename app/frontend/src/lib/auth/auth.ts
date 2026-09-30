'use server'

import { redirect } from 'next/navigation'
import { createSession, deleteSession } from './session'
import apiService from '../api/root'

export async function signOut() {
  await deleteSession()
  redirect('/')
}

export async function signIn(payload: { email: string; password: string }): Promise<{ ok: boolean; message?: string }> {
  try {
    const data = await apiService('POST', 'auth/login', payload, false)

    if (!data.token) {
      return { ok: false, message: 'Đăng nhập thất bại' }
    }

    await createSession({
      access_token: data.token,
      role: data.role
    })

    return { ok: true }
  } catch (error: any) {
    return { ok: false, message: error?.message || 'Email hoặc mật khẩu không đúng' }
  }
}
