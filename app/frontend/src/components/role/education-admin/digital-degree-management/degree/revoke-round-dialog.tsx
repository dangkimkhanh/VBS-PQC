'use client'

import { useState } from 'react'
import { AlertTriangle, Ban } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import RoundPicker from '@/components/common/round-picker'
import { UseData } from '@/components/providers/data-provider'
import { revokeIssuanceRound } from '@/lib/api/digital-degree'
import { formatFacultyOptionsByID } from '@/lib/utils/format-api'
import { showNotification } from '@/lib/utils/common'

const ALL_FACULTIES = 'all'

interface Props {
  // Faculty and round of the current search, used as starting values that the user may change.
  facultyId?: string
  roundId?: string
  roundName?: string
  onRevoked: () => void
}

// Revokes every issued diploma of one faculty in one issuance round, for example
// when a whole round was issued by mistake.
const RevokeRoundDialog: React.FC<Props> = ({ facultyId, roundId: searchRoundId, roundName: searchRoundName, onRevoked }) => {
  const facultyOptions = formatFacultyOptionsByID(UseData().facultyList)
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [faculty, setFaculty] = useState('')
  const [roundId, setRoundId] = useState('')
  const [roundName, setRoundName] = useState('')
  const [reason, setReason] = useState('')
  const [confirm, setConfirm] = useState('')

  const reset = () => {
    // A round searched without a faculty covers every faculty of that round.
    const round = searchRoundId && searchRoundId !== 'none' && searchRoundName ? searchRoundId : ''
    setFaculty(facultyId && facultyId !== ALL_FACULTIES ? facultyId : round ? ALL_FACULTIES : '')
    setRoundId(round)
    setRoundName(round ? (searchRoundName as string) : '')
    setReason('')
    setConfirm('')
  }

  const normalized = (value: string) => value.trim().replace(/\s+/g, ' ').toLowerCase()
  const confirmed = roundName !== '' && normalized(confirm) === normalized(roundName)
  const reasonOk = reason.trim().length >= 5 && reason.trim().length <= 500
  const facultyLabel =
    faculty === ALL_FACULTIES ? 'tất cả chuyên ngành' : facultyOptions.find((o: any) => o.value === faculty)?.label

  const submit = async () => {
    setBusy(true)
    try {
      const res = await revokeIssuanceRound({ faculty_id: faculty, round_id: roundId, reason: reason.trim(), confirm })
      showNotification('success', `Đã thu hồi ${res.revoked} văn bằng của đợt "${roundName}"`)
      setOpen(false)
      reset()
      onRevoked()
    } catch (error: any) {
      showNotification('error', error?.message || 'Không thể thu hồi văn bằng')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (busy) return
        setOpen(v)
        if (v) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button variant='outline' className='text-red-600 hover:text-red-700'>
          <Ban />
          <span className='hidden md:block'>Thu hồi</span>
        </Button>
      </DialogTrigger>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-[560px]'>
        <DialogHeader>
          <DialogTitle>Thu hồi văn bằng theo đợt cấp</DialogTitle>
          <DialogDescription>
            Thu hồi các văn bằng đã cấp của một đợt: chọn một chuyên ngành, hoặc tất cả chuyên ngành của đợt. Để thu hồi một văn bằng riêng lẻ, dùng nút
            thu hồi ở từng dòng trong bảng.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-2'>
          <Label>Chuyên ngành*</Label>
          <Select
            value={faculty}
            onValueChange={(v) => {
              setFaculty(v)
              setRoundId('')
              setRoundName('')
              setConfirm('')
            }}
            disabled={busy}
          >
            <SelectTrigger>
              <SelectValue placeholder='Chọn chuyên ngành' />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL_FACULTIES}>Tất cả chuyên ngành</SelectItem>
              {facultyOptions.map((o: any) => (
                <SelectItem key={o.value} value={o.value}>
                  {o.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className='space-y-2'>
          <Label>Đợt cấp*</Label>
          <RoundPicker
            value={roundId}
            valueLabel={roundName}
            facultyId={faculty === ALL_FACULTIES ? '' : faculty}
            disabled={!faculty || busy}
            placeholder={faculty ? 'Chọn đợt cấp' : 'Chọn chuyên ngành trước'}
            onChange={(id, round) => {
              setRoundId(id)
              setRoundName(round?.name || '')
              setConfirm('')
            }}
          />
        </div>

        <div className='space-y-2'>
          <Label htmlFor='revoke-reason'>Lý do thu hồi*</Label>
          <Textarea
            id='revoke-reason'
            rows={3}
            maxLength={500}
            placeholder='Ví dụ: Cấp nhầm danh sách tốt nghiệp theo quyết định số ...'
            value={reason}
            disabled={busy}
            onChange={(e) => setReason(e.target.value)}
          />
        </div>

        {roundId && (
          <Alert variant='destructive'>
            <AlertTriangle />
            <AlertTitle>Thao tác không hoàn tác được</AlertTitle>
            <AlertDescription className='space-y-2'>
              <p>
                Mọi văn bằng đã cấp thuộc <strong>{facultyLabel}</strong> trong đợt <strong>{roundName}</strong> sẽ bị thu
                hồi. Trang xác minh công khai sẽ báo các văn bằng này đã thu hồi.
              </p>
              <Label htmlFor='revoke-confirm'>Nhập lại tên đợt để xác nhận</Label>
              <Input
                id='revoke-confirm'
                placeholder={roundName}
                value={confirm}
                disabled={busy}
                onChange={(e) => setConfirm(e.target.value)}
              />
            </AlertDescription>
          </Alert>
        )}

        <DialogFooter>
          <DialogClose asChild>
            <Button variant='outline' disabled={busy}>
              Hủy
            </Button>
          </DialogClose>
          <Button variant='destructive' disabled={!faculty || !roundId || !reasonOk || !confirmed} isLoading={busy} onClick={submit}>
            <Ban /> Thu hồi
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default RevokeRoundDialog
