'use client'

import { useState } from 'react'
import { FilePlus2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { createDigitalDegree } from '@/lib/api/digital-degree'
import { showNotification } from '@/lib/utils/common'
import RoundField, { emptyRoundChoice, resolveRoundChoice, roundChoiceError, RoundChoice } from './round-field'

interface Props {
  onCreated: () => void
}

const CreateDigitalDegreeDialog: React.FC<Props> = ({ onCreated }) => {
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({
    student_code: '',
    name: 'Bằng tốt nghiệp',
    certificate_type: '',
    course: '',
    education_type: '',
    graduation_rank: '',
    gpa: '',
    issue_date: new Date().toISOString().slice(0, 10),
    serial_number: '',
    registration_number: ''
  })

  const [round, setRound] = useState<RoundChoice>(emptyRoundChoice)
  const update = (field: string, value: string) => setForm((current) => ({ ...current, [field]: value }))

  const handleCreate = async () => {
    const required = ['student_code', 'name', 'issue_date', 'serial_number', 'registration_number']
    if (required.some((field) => !String(form[field as keyof typeof form]).trim())) {
      showNotification('error', 'Vui lòng nhập đủ mã sinh viên, tên bằng, ngày cấp, số hiệu và số vào sổ')
      return
    }
    const problem = roundChoiceError(round)
    if (problem) {
      showNotification('error', problem)
      return
    }
    try {
      setLoading(true)
      const roundId = await resolveRoundChoice(round)
      // A newly created round is kept selected for the next record of the same list.
      setRound({ ...emptyRoundChoice, roundId, roundLabel: round.mode === 'new' ? round.name.trim() : round.roundLabel })
      await createDigitalDegree({
        ...form,
        gpa: form.gpa ? Number(form.gpa) : 0,
        round_id: roundId
      })
      showNotification('success', 'Đã tạo hồ sơ văn bằng số ở trạng thái Chưa cấp')
      setOpen(false)
      setForm((current) => ({ ...current, student_code: '', serial_number: '', registration_number: '' }))
      onCreated()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể tạo hồ sơ văn bằng số')
    } finally {
      setLoading(false)
    }
  }

  const fields = [
    ['student_code', 'Mã sinh viên*'],
    ['name', 'Tên văn bằng*'],
    ['certificate_type', 'Loại bằng'],
    ['course', 'Khóa học'],
    ['education_type', 'Hình thức đào tạo'],
    ['graduation_rank', 'Xếp loại'],
    ['gpa', 'GPA'],
    ['issue_date', 'Ngày cấp*'],
    ['serial_number', 'Số hiệu*'],
    ['registration_number', 'Số vào sổ*']
  ] as const

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant='outline'><FilePlus2 /> <span className='hidden md:block'>Thêm văn bằng</span></Button>
      </DialogTrigger>
      <DialogContent className='max-h-[85vh] overflow-y-auto sm:max-w-[560px]'>
        <DialogHeader>
          <DialogTitle>Tạo hồ sơ văn bằng số</DialogTitle>
          <DialogDescription>
            Tạo một hồ sơ cho từng sinh viên. Excel chỉ dùng khi cần nhập nhiều văn bằng cùng lúc.
          </DialogDescription>
        </DialogHeader>
        <RoundField value={round} onChange={setRound} disabled={loading} />
        <div className='grid gap-4 sm:grid-cols-2'>
          {fields.map(([field, label]) => (
            <div className='space-y-2' key={field}>
              <Label htmlFor={field}>{label}</Label>
              <Input
                id={field}
                type={field === 'issue_date' ? 'date' : field === 'gpa' ? 'number' : 'text'}
                step={field === 'gpa' ? '0.01' : undefined}
                value={form[field]}
                onChange={(event) => update(field, event.target.value)}
              />
            </div>
          ))}
        </div>
        <DialogFooter>
          <DialogClose asChild><Button variant='outline'>Hủy</Button></DialogClose>
          <Button onClick={handleCreate} isLoading={loading}><FilePlus2 /> Tạo hồ sơ</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default CreateDigitalDegreeDialog
