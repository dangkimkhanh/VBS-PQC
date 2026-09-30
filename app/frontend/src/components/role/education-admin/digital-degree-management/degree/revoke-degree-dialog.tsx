'use client'

import { useState } from 'react'
import { Ban } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from '@/components/ui/alert-dialog'
import { revokeDigitalDegree } from '@/lib/api/digital-degree'
import { showNotification } from '@/lib/utils/common'

interface Props {
  diplomaId: string
  onRevoked: () => void
}

const RevokeDegreeDialog = ({ diplomaId, onRevoked }: Props) => {
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)

  const revoke = async () => {
    if (reason.trim().length < 5) {
      showNotification('error', 'Vui lòng nhập lý do thu hồi (ít nhất 5 ký tự)')
      return
    }
    setLoading(true)
    try {
      await revokeDigitalDegree(diplomaId, reason.trim())
      showNotification('success', 'Đã thu hồi văn bằng số')
      onRevoked()
      setOpen(false)
      setReason('')
    } catch (error: any) {
      showNotification('error', error?.message || 'Không thể thu hồi văn bằng')
    } finally {
      setLoading(false)
    }
  }

  return (
    <AlertDialog
      open={open}
      onOpenChange={(value) => {
        if (!loading) setOpen(value)
      }}
    >
      <AlertDialogTrigger asChild>
        <Button size='icon' variant='outline' title='Thu hồi văn bằng số'>
          <Ban className='h-4 w-4 text-red-600' />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Thu hồi văn bằng số?</AlertDialogTitle>
          <AlertDialogDescription>
            Văn bằng sẽ chuyển sang trạng thái không còn hiệu lực. Thao tác này không xóa dữ liệu và không thể cấp lại
            chính văn bằng đã thu hồi.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <Textarea placeholder='Nhập lý do thu hồi' value={reason} onChange={(event) => setReason(event.target.value)} />
        <AlertDialogFooter>
          <AlertDialogCancel>Hủy</AlertDialogCancel>
          <AlertDialogAction
            onClick={(event) => {
              event.preventDefault()
              void revoke()
            }}
            disabled={loading || reason.trim().length < 5}
          >
            {loading ? 'Đang xử lý...' : 'Xác nhận thu hồi'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export default RevokeDegreeDialog
