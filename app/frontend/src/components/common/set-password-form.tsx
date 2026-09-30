'use client'

import { Suspense, useState } from 'react'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { showNotification } from '@/lib/utils/common'

interface Props {
  title: string
  description: string
  submitLabel: string
  successMessage: string
  // eslint-disable-next-line no-unused-vars
  submit: (token: string, password: string) => Promise<unknown>
}

// Shared by the activation and reset-password pages: both exchange a one-time
// emailed token for a password chosen by the user.
const SetPasswordFormInner: React.FC<Props> = (props) => {
  const token = useSearchParams().get('token') || ''
  const router = useRouter()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [loading, setLoading] = useState(false)
  const mismatch = confirm !== '' && confirm !== password

  const onSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (mismatch) return
    setLoading(true)
    try {
      await props.submit(token, password)
      showNotification('success', props.successMessage)
      router.push('/auth/sign-in')
    } catch (error: any) {
      showNotification('error', error.message || 'Thao tác không thành công')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className='flex min-h-screen items-center justify-center bg-slate-50 px-4 dark:bg-background'>
      <section className='w-full max-w-md rounded-xl bg-white p-6 shadow-sm dark:bg-card'>
        <h1 className='text-2xl font-semibold'>{props.title}</h1>
        <p className='mb-6 mt-2 text-sm text-muted-foreground'>{props.description}</p>
        {!token ? (
          <p className='text-sm text-destructive'>
            Liên kết không hợp lệ. Vui lòng mở đúng liên kết trong email hoặc yêu cầu liên kết mới.
          </p>
        ) : (
          <form onSubmit={onSubmit} className='space-y-4'>
            <Input
              type='password'
              autoComplete='new-password'
              minLength={8}
              maxLength={72}
              placeholder='Mật khẩu mới (tối thiểu 8 ký tự)'
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
            <Input
              type='password'
              autoComplete='new-password'
              placeholder='Nhập lại mật khẩu mới'
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
            />
            {mismatch && <p className='text-sm text-destructive'>Mật khẩu nhập lại không khớp</p>}
            <Button type='submit' className='w-full' isLoading={loading} disabled={mismatch}>
              {props.submitLabel}
            </Button>
          </form>
        )}
        <Link href='/auth/sign-in' className='mt-6 block text-center text-sm underline'>
          Về trang đăng nhập
        </Link>
      </section>
    </main>
  )
}

const SetPasswordForm: React.FC<Props> = (props) => (
  <Suspense>
    <SetPasswordFormInner {...props} />
  </Suspense>
)

export default SetPasswordForm
