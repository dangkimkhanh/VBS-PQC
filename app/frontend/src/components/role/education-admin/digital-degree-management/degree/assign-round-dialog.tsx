'use client'

import { useState } from 'react'
import { Layers } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from '@/components/ui/dialog'
import { assignIssuanceRound } from '@/lib/api/digital-degree'
import { showNotification } from '@/lib/utils/common'
import RoundField, { emptyRoundChoice, resolveRoundChoice, roundChoiceError, RoundChoice } from './round-field'

interface Props {
  facultyId: string
  facultyLabel?: string
  course?: string
  certificateType?: string
  onAssigned: () => void
}

// Attaches records created before issuance rounds existed to a round, so they can
// be issued, signed, anchored and revoked like the others.
const AssignRoundDialog: React.FC<Props> = (props) => {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [round, setRound] = useState<RoundChoice>(emptyRoundChoice)

  const submit = async () => {
    const problem = roundChoiceError(round)
    if (problem) {
      showNotification('error', problem)
      return
    }
    setBusy(true)
    try {
      const roundId = await resolveRoundChoice(round)
      const res = await assignIssuanceRound({
        round_id: roundId,
        faculty_id: props.facultyId || 'all',
        course: props.course || undefined,
        certificate_type: props.certificateType || undefined
      })
      showNotification('success', `Đã gán đợt cấp cho ${res.updated} hồ sơ`)
      setOpen(false)
      setRound(emptyRoundChoice)
      props.onAssigned()
    } catch (error: any) {
      showNotification('error', error?.message || 'Không thể gán đợt cấp')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !busy && setOpen(v)}>
      <DialogTrigger asChild>
        <Button size='sm'>
          <Layers /> Gán đợt cấp
        </Button>
      </DialogTrigger>
      <DialogContent className='sm:max-w-[520px]'>
        <DialogHeader>
          <DialogTitle>Gán đợt cấp cho hồ sơ chưa có đợt</DialogTitle>
          <DialogDescription>
            Áp dụng cho các hồ sơ chưa gắn đợt thuộc {props.facultyLabel || 'tất cả chuyên ngành'}
            {props.course ? `, khóa ${props.course}` : ''}
            {props.certificateType ? `, loại bằng ${props.certificateType}` : ''}.
          </DialogDescription>
        </DialogHeader>
        <RoundField value={round} onChange={setRound} disabled={busy} />
        <DialogFooter>
          <DialogClose asChild>
            <Button variant='outline' disabled={busy}>
              Hủy
            </Button>
          </DialogClose>
          <Button onClick={submit} isLoading={busy}>
            Gán đợt
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default AssignRoundDialog
