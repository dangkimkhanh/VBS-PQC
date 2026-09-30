'use client'

import { useEffect, useState } from 'react'
import { FileCheck2, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import CommonSelect from '../../common-select'
import { formatDegreeTemplateOptions } from '@/lib/utils/format-api'
import { issueDigitalDegree, searchDegreeTemplateByFaculty } from '@/lib/api/digital-degree'
import { signPQCEDiploma } from '@/lib/api/pqc'
import { showMessage, showNotification } from '@/lib/utils/common'

interface Props {
  diplomaId: string
  facultyId: string
  studentName?: string
  onIssued: () => void
}

const IssueSingleDegreeDialog: React.FC<Props> = ({ diplomaId, facultyId, studentName, onIssued }) => {
  const [open, setOpen] = useState(false)
  const [loadingTemplates, setLoadingTemplates] = useState(false)
  const [loading, setLoading] = useState(false)
  const [templateId, setTemplateId] = useState('')
  const [templates, setTemplates] = useState<any[]>([])

  useEffect(() => {
    if (!open || !facultyId) return
    let cancelled = false
    setLoadingTemplates(true)
    searchDegreeTemplateByFaculty(facultyId)
      .then((response) => {
        if (!cancelled) setTemplates(response.data || [])
      })
      .catch((error: any) => showNotification('error', error.message || 'Không thể tải danh sách mẫu bằng'))
      .finally(() => {
        if (!cancelled) setLoadingTemplates(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, facultyId])

  const handleIssue = async () => {
    if (!templateId) {
      showMessage('Vui lòng chọn mẫu bằng')
      return
    }
    try {
      setLoading(true)
      await issueDigitalDegree(diplomaId, templateId)
      await signPQCEDiploma(diplomaId)
      showNotification('success', `Đã cấp và ký ML-DSA văn bằng cho ${studentName || 'sinh viên'}`)
      setOpen(false)
      setTemplateId('')
      onIssued()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể cấp văn bằng cho sinh viên này')
    } finally {
      setLoading(false)
      onIssued()
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size='icon' variant='outline' title='Cấp lẻ văn bằng cho sinh viên'>
          <FileCheck2 />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Cấp lẻ văn bằng số</DialogTitle>
          <DialogDescription>
            Sinh PDF từ mẫu bằng và ký ML-DSA cho <strong>{studentName || 'sinh viên này'}</strong>.
          </DialogDescription>
        </DialogHeader>
        {loadingTemplates ? (
          <div className='flex items-center gap-2 text-sm text-muted-foreground'><Loader2 className='animate-spin' /> Đang tải mẫu bằng...</div>
        ) : (
          <CommonSelect
            value={templateId}
            options={formatDegreeTemplateOptions(templates)}
            handleSelect={setTemplateId}
            placeholder='Chọn mẫu bằng'
          />
        )}
        <DialogFooter>
          <DialogClose asChild><Button variant='outline'>Hủy</Button></DialogClose>
          <Button onClick={handleIssue} isLoading={loading} disabled={loadingTemplates || !templateId}>
            <FileCheck2 /> Cấp và ký
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default IssueSingleDegreeDialog
