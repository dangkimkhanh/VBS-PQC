'use client'

import { useState } from 'react'
import Link from 'next/link'
import { requestPasswordReset } from '@/lib/api/auth'
import { showNotification } from '@/lib/utils/common'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [sentMessage, setSentMessage] = useState('')

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    setLoading(true)
    try {
      const result = await requestPasswordReset(email.trim())
      setSentMessage(result.message || 'Nếu email tồn tại, hướng dẫn đã được gửi.')
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể xử lý yêu cầu')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className='flex min-h-screen items-center justify-center bg-slate-50 px-4 dark:bg-background'>
      <section className='w-full max-w-md rounded-xl bg-white p-6 shadow-sm dark:bg-card'>
        <h1 className='text-2xl font-semibold'>Quên mật khẩu</h1>
        {sentMessage ? (
          <p className='mt-4 text-sm'>
            {sentMessage} Liên kết có hiệu lực 20 phút. Hãy kiểm tra cả thư mục thư rác.
          </p>
        ) : (
          <>
            <p className='mb-6 mt-2 text-sm text-muted-foreground'>
              Nhập email đăng nhập. Sinh viên chưa kích hoạt tài khoản hãy dùng chức năng kích hoạt bằng OTP.
            </p>
            <form onSubmit={submit} className='space-y-4'>
              <Input
                type='email'
                placeholder='Email đăng nhập'
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
              />
              <Button type='submit' className='w-full' isLoading={loading}>
                Gửi liên kết đặt lại mật khẩu
              </Button>
            </form>
          </>
        )}
        <Link href='/auth/sign-in' className='mt-6 block text-center text-sm underline'>
          Về trang đăng nhập
        </Link>
      </section>
    </main>
  )
}
