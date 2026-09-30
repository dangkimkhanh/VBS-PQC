'use client'

import { useRef, useState } from 'react'
import { FileSpreadsheet, UploadIcon } from 'lucide-react'
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
import { Label } from '@/components/ui/label'
import { showNotification } from '@/lib/utils/common'
import RoundField, { emptyRoundChoice, resolveRoundChoice, roundChoiceError, RoundChoice } from './round-field'

interface Props {
  // Uploads the prepared form (file + round_id); resolves when the import finished.
  // eslint-disable-next-line no-unused-vars
  onSubmit: (data: FormData) => Promise<unknown>
}

// Imports one Excel list of graduates into one issuance round.
const ImportDegreeExcelDialog: React.FC<Props> = ({ onSubmit }) => {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [round, setRound] = useState<RoundChoice>(emptyRoundChoice)
  const inputRef = useRef<HTMLInputElement>(null)

  const reset = () => {
    setFile(null)
    setRound(emptyRoundChoice)
    if (inputRef.current) inputRef.current.value = ''
  }

  const submit = async () => {
    if (!file) {
      showNotification('error', 'Vui lòng chọn tệp Excel')
      return
    }
    const problem = roundChoiceError(round)
    if (problem) {
      showNotification('error', problem)
      return
    }
    setBusy(true)
    try {
      const roundId = await resolveRoundChoice(round)
      const data = new FormData()
      data.append('file', file)
      data.append('round_id', roundId)
      await onSubmit(data)
      setOpen(false)
      reset()
    } catch (error: any) {
      showNotification('error', error?.message || 'Không thể tải tệp lên')
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
        if (!v) reset()
      }}
    >
      <DialogTrigger asChild>
        <Button variant='outline'>
          <UploadIcon />
          <span className='hidden md:block'>Tải Excel lên</span>
        </Button>
      </DialogTrigger>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-[560px]'>
        <DialogHeader>
          <DialogTitle>Nhập danh sách văn bằng từ Excel</DialogTitle>
          <DialogDescription>
            Toàn bộ danh sách trong tệp được gắn vào một đợt cấp. Các thao tác cấp, ký, ghi Blockchain và thu hồi về sau
            đều chọn theo đợt này.
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-2'>
          <Label htmlFor='degree-excel'>Tệp Excel*</Label>
          <input
            ref={inputRef}
            id='degree-excel'
            type='file'
            accept='.xlsx,.xls'
            className='hidden'
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <Button
            type='button'
            variant='outline'
            className='w-full justify-start font-normal'
            disabled={busy}
            onClick={() => inputRef.current?.click()}
          >
            <FileSpreadsheet />
            <div className='truncate'>{file ? file.name : 'Chọn tệp .xlsx'}</div>
          </Button>
        </div>

        <RoundField value={round} onChange={setRound} disabled={busy} />

        <DialogFooter>
          <DialogClose asChild>
            <Button variant='outline' disabled={busy}>
              Hủy
            </Button>
          </DialogClose>
          <Button onClick={submit} isLoading={busy}>
            <UploadIcon /> Tải lên
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default ImportDegreeExcelDialog
