'use client'

import { useEffect, useState } from 'react'
import { Mail } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { resendUniversityActivation } from '@/lib/api/university'
import { showNotification } from '@/lib/utils/common'

interface Props {
  id: string
  // From the API (activation_resend_at): each new link cancels the previous one,
  // so the backend only allows a resend a few minutes after the last email.
  resendAt?: string
  onSent?: () => void
  compact?: boolean
}

const ResendActivationButton: React.FC<Props> = ({ id, resendAt, onSent, compact }) => {
  const [now, setNow] = useState(() => Date.now())
  const [busy, setBusy] = useState(false)
  const waitMs = resendAt ? Date.parse(resendAt) - now : 0
  const waiting = waitMs > 0

  useEffect(() => {
    if (!waiting) return
    const timer = setInterval(() => setNow(Date.now()), 15_000)
    return () => clearInterval(timer)
  }, [waiting])

  const minutes = Math.max(1, Math.ceil(waitMs / 60_000))
  const label = waiting ? `Gửi lại sau ${minutes} phút` : 'Gửi lại link kích hoạt'

  const send = async () => {
    setBusy(true)
    try {
      const res = await resendUniversityActivation(id)
      showNotification('success', res?.message || 'Đã gửi lại link kích hoạt')
      onSent?.()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể gửi lại link')
    } finally {
      setBusy(false)
      setNow(Date.now())
    }
  }

  if (compact) {
    return (
      <Button size='icon' variant='outline' title={label} aria-label={label} disabled={busy || waiting} onClick={send}>
        <Mail />
      </Button>
    )
  }
  return (
    <Button size='sm' variant='outline' disabled={busy || waiting} onClick={send}>
      <Mail /> {waiting ? label : 'Gửi lại link'}
    </Button>
  )
}

export default ResendActivationButton
