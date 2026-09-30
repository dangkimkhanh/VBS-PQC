'use client'
import { useRef, useState } from 'react'
import useSWR, { mutate } from 'swr'
import { toast } from 'sonner'
import { KeyRound } from 'lucide-react'
import apiService from '@/lib/api/root'
import { searchDegreeTemplateByFaculty } from '@/lib/api/digital-degree'
import { getUserFriendlyErrorMessage, showNotification } from '@/lib/utils/common'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger
} from '@/components/ui/dialog'

interface Props {
  filter: { faculty_id?: string; round_id?: string; certificate_type?: string; course?: string; issued?: string }
  // Names of the filtered faculty and round, shown in the dialog and in the background notification.
  facultyName?: string
  roundName?: string
  onSigned: () => void
}

interface Target {
  id: string
  student_code: string
}

interface Progress {
  done: number
  total: number
  signed: number
  failed: number
  firstError?: string
  finished: boolean
}

interface Job {
  id: number
  action: string
  label: string
  progress: Progress
  // Set when the dialog is closed during the run: the progress then lives in a notification.
  detached: boolean
}

// "Chưa cấp" filter: issue and sign the unissued records.
// "Đã cấp" filter: only sign the issued records that were never signed.
const stageOf = (issued?: string) => (issued === 'false' ? 'issue' : issued === 'true' ? 'sign' : null)

const toastId = (job: Job) => `sign-batch-${job.id}`

const SignButton: React.FC<Props> = ({ filter, facultyName, roundName, onSigned }) => {
  const scopeLabel = [facultyName, roundName].filter(Boolean).join(' · ')
  const [open, setOpen] = useState(false)
  const [template, setTemplate] = useState('')
  const [error, setError] = useState('')
  // Progress of the batch shown in the dialog; other batches run on in the background.
  const [progress, setProgress] = useState<Progress | null>(null)
  const dialogJob = useRef<Job | null>(null)
  const nextJobId = useRef(0)
  // Records taken by a running batch, so a second batch never processes them again.
  const claimed = useRef(new Set<string>())
  const [, setClaimedVersion] = useState(0)

  const stage = stageOf(filter.issued)
  const hasRound = Boolean(filter.round_id && filter.round_id !== 'none')
  const hasFaculty = Boolean(filter.faculty_id && filter.faculty_id !== 'all')
  // The diploma template belongs to a faculty, so issuing needs one faculty; signing does not.
  const ready = Boolean(stage && hasRound && (stage === 'sign' || hasFaculty))
  const scope = {
    faculty_id: filter.faculty_id,
    round_id: filter.round_id,
    course: filter.course,
    certificate_type: filter.certificate_type,
    stage
  }
  const targetsKey = ready
    ? ['batch-targets', stage, filter.faculty_id, filter.round_id, filter.course, filter.certificate_type]
    : null
  const targets = useSWR(targetsKey, () => apiService('POST', 'ediplomas/issue-pqc-batch', { ...scope, dry_run: true }))
  const scopeTargets: Target[] = targets.data?.data ?? []
  const count = scopeTargets.filter((t) => !claimed.current.has(t.id)).length
  const busyInScope = scopeTargets.length - count
  const templates = useSWR(
    open && stage === 'issue' && hasFaculty ? ['batch-templates', filter.faculty_id] : null,
    () => searchDegreeTemplateByFaculty(filter.faculty_id as string)
  )

  const label = stage === 'sign' ? 'Ký ML-DSA' : 'Cấp & ký'
  const action = stage === 'sign' ? 'ký' : 'cấp và ký'
  const hint = !stage
    ? 'Chọn trạng thái Chưa cấp hoặc Đã cấp trong bộ lọc'
    : !hasRound
      ? 'Chọn đợt cấp trong bộ lọc'
      : stage === 'issue' && !hasFaculty
        ? 'Chọn một chuyên ngành trong bộ lọc'
        : count === 0
          ? busyInScope > 0
            ? 'Các văn bằng của phạm vi này đang được ký'
            : stage === 'sign'
              ? 'Không có văn bằng đã cấp nào chưa ký'
              : 'Không có văn bằng chưa cấp'
          : undefined

  const showRunning = (job: Job) => {
    const p = job.progress
    toast.loading(`Đang ${job.action} ${p.done}/${p.total} văn bằng`, {
      id: toastId(job),
      description: job.label || undefined,
      duration: Infinity
    })
  }

  const showFinished = (job: Job) => {
    const p = job.progress
    const text = `Đã ${job.action} ${p.signed}/${p.total} văn bằng${job.label ? ` · ${job.label}` : ''}`
    const setting = { id: toastId(job), duration: 6000 }
    if (p.failed === 0) {
      showNotification('success', text, setting)
    } else {
      showNotification(
        'info',
        `${text}; ${p.failed} văn bằng lỗi: ${getUserFriendlyErrorMessage(p.firstError ?? '')}`,
        setting
      )
    }
  }

  const report = (job: Job) => {
    if (job.detached) showRunning(job)
    else if (dialogJob.current === job) setProgress({ ...job.progress })
  }

  const run = async () => {
    setError('')
    let list: Target[]
    try {
      list = (await apiService('POST', 'ediplomas/issue-pqc-batch', { ...scope, dry_run: true })).data
    } catch (e: any) {
      setError(e.message)
      return
    }
    list = list.filter((t) => !claimed.current.has(t.id))
    if (list.length === 0) {
      setError('Các văn bằng của phạm vi này đang được ký hoặc đã được ký.')
      return
    }
    const job: Job = {
      id: ++nextJobId.current,
      action,
      label: scopeLabel,
      detached: false,
      progress: { done: 0, total: list.length, signed: 0, failed: 0, finished: false }
    }
    const jobScope = { ...scope, template_id: template }
    list.forEach((t) => claimed.current.add(t.id))
    setClaimedVersion((v) => v + 1)
    dialogJob.current = job
    report(job)

    // One record per request, so the progress is exact and a failure never stops the rest.
    for (const target of list) {
      try {
        const r = await apiService('POST', 'ediplomas/issue-pqc-batch', { ...jobScope, ids: [target.id] })
        const row = r.data?.[0]
        if (row?.signed) {
          job.progress.signed++
        } else {
          job.progress.failed++
          job.progress.firstError ??= row?.error ?? 'Văn bằng không còn trong phạm vi xử lý'
        }
      } catch (e: any) {
        job.progress.failed++
        job.progress.firstError ??= e.message
      }
      job.progress.done++
      report(job)
    }
    job.progress.finished = true
    if (job.detached) showFinished(job)
    else report(job)
    onSigned()
    // Release the records only once the fresh list no longer counts them as waiting.
    await targets.mutate()
    list.forEach((t) => claimed.current.delete(t.id))
    setClaimedVersion((v) => v + 1)
    mutate('degree-summary')
    mutate('degree-audit')
  }

  const reset = () => {
    dialogJob.current = null
    setProgress(null)
    setTemplate('')
    setError('')
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        setOpen(v)
        if (v) {
          targets.mutate()
          return
        }
        // Closing during a run moves its progress to a notification and frees the dialog for another batch.
        const job = dialogJob.current
        if (job && !job.progress.finished) {
          job.detached = true
          showRunning(job)
        }
        reset()
      }}
    >
      <DialogTrigger asChild>
        <Button variant='secondary' disabled={Boolean(hint)} title={hint}>
          <KeyRound />
          <span className='hidden md:block'>{label}</span>
        </Button>
      </DialogTrigger>
      <DialogContent className='max-h-[85vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>{stage === 'sign' ? 'Ký ML-DSA văn bằng đã cấp' : 'Cấp và ký ML-DSA'}</DialogTitle>
          <DialogDescription>
            {progress
              ? scopeLabel
              : stage === 'sign'
                ? `Ký các văn bằng đã cấp nhưng chưa ký của đợt đang lọc (${count} văn bằng).`
                : `Cấp PDF và ký ML-DSA các văn bằng chưa cấp của chuyên ngành và đợt đang lọc (${count} văn bằng).`}
          </DialogDescription>
        </DialogHeader>

        {!progress && (
          <>
            <div className='space-y-1 rounded-lg bg-muted p-3 text-sm'>
              <p>
                Chuyên ngành: <strong>{facultyName || 'Tất cả chuyên ngành'}</strong>
              </p>
              <p>
                Đợt cấp: <strong>{roundName || 'đã chọn trong bộ lọc'}</strong>
              </p>
              <p>
                Khóa học: {filter.course || 'Tất cả'} · Loại bằng: {filter.certificate_type || 'Tất cả'}
              </p>
            </div>
            {stage === 'issue' && (
              <label className='text-sm font-medium'>
                Mẫu văn bằng
                <select
                  className='mt-2 w-full rounded-lg border bg-background p-3'
                  value={template}
                  onChange={(e) => setTemplate(e.target.value)}
                >
                  <option value=''>Chọn mẫu thuộc chuyên ngành</option>
                  {templates.data?.data?.map((t: any) => (
                    <option key={t.id} value={t.id}>
                      {t.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {templates.error && (
              <p role='alert' className='text-red-600 dark:text-red-400'>
                Không tải được mẫu bằng.
              </p>
            )}
            {error && (
              <p role='alert' className='text-red-600 dark:text-red-400'>
                {getUserFriendlyErrorMessage(error)}
              </p>
            )}
            <Button disabled={count === 0 || (stage === 'issue' && !template)} onClick={run}>
              Xác nhận {action}
            </Button>
          </>
        )}

        {progress && (
          <div role='status' className='space-y-3'>
            <div className='flex items-baseline justify-between text-sm'>
              <span className='font-medium'>
                {progress.finished ? 'Hoàn thành' : `Đang ${dialogJob.current?.action ?? action}…`}
              </span>
              <span className='tabular-nums'>
                {progress.done}/{progress.total}
              </span>
            </div>
            <div className='h-2.5 w-full overflow-hidden rounded-full bg-muted'>
              <div
                className='h-full rounded-full bg-primary transition-all'
                style={{ width: `${progress.total ? (progress.done / progress.total) * 100 : 100}%` }}
              />
            </div>
            <p className='text-sm text-muted-foreground'>
              Đã ký {progress.signed} văn bằng
              {progress.failed > 0 && `, ${progress.failed} văn bằng lỗi`}.
              {!progress.finished && ' Có thể đóng hộp thoại, tiến độ sẽ hiện ở khu thông báo.'}
            </p>
            {progress.finished && progress.firstError && (
              <p role='alert' className='text-sm text-red-600 dark:text-red-400'>
                {getUserFriendlyErrorMessage(progress.firstError)}
              </p>
            )}
            <Button
              className='w-full'
              disabled={!progress.finished}
              onClick={() => {
                setOpen(false)
                reset()
              }}
            >
              Hoàn tất
            </Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}

export default SignButton
