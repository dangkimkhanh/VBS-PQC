'use client'

import { useEffect, useState } from 'react'
import { AlertTriangle, CheckCircle2, KeyRound, Plus, RefreshCw, ShieldCheck, ShieldX } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import {
  activatePQCKey,
  createPQCKey,
  DEFAULT_PQC_ALGORITHM,
  getPQCKeys,
  PQC_ALGORITHMS,
  PQCAlgorithm,
  PQCKeyRecord,
  revokePQCKey
} from '@/lib/api/pqc'
import { cn, showNotification } from '@/lib/utils/common'

const statusLabel: Record<PQCKeyRecord['status'], string> = {
  pending: 'Chờ kích hoạt',
  active: 'Đang hoạt động',
  retired: 'Đã luân chuyển',
  revoked: 'Đã thu hồi'
}

const KeyManagement = () => {
  const [keys, setKeys] = useState<PQCKeyRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [newKeyName, setNewKeyName] = useState('Khóa ký văn bằng')
  const [newKeyAlgorithm, setNewKeyAlgorithm] = useState<PQCAlgorithm>(DEFAULT_PQC_ALGORITHM)
  const activeKey = keys.find((key) => key.status === 'active')
  const [revokeTarget, setRevokeTarget] = useState<PQCKeyRecord | null>(null)
  const [revokeReason, setRevokeReason] = useState('')
  const [compromisedAt, setCompromisedAt] = useState('')

  const loadKeys = async () => {
    try {
      setLoading(true)
      setKeys(await getPQCKeys())
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể tải danh sách khóa')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadKeys()
  }, [])

  const createKey = async () => {
    if (newKeyName.trim().length < 3) {
      showNotification('error', 'Tên khóa cần ít nhất 3 ký tự')
      return
    }
    try {
      setBusyId('create')
      await createPQCKey(newKeyName.trim(), newKeyAlgorithm)
      showNotification('success', `Đã tạo khóa ${newKeyAlgorithm}. Hãy kích hoạt trước khi ký.`)
      await loadKeys()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể tạo khóa')
    } finally {
      setBusyId(null)
    }
  }

  const activateKey = async (key: PQCKeyRecord) => {
    if (
      activeKey &&
      activeKey.algorithm !== key.algorithm &&
      !window.confirm(
        `Kích hoạt khóa này sẽ chuyển thuật toán ký văn bằng mới từ ${activeKey.algorithm} sang ${key.algorithm}. Văn bằng đã ký trước đó vẫn được xác minh bình thường. Tiếp tục?`
      )
    ) {
      return
    }
    try {
      setBusyId(key.id)
      await activatePQCKey(key.id)
      showNotification('success', 'Đã kích hoạt khóa. Khóa cũ ngừng dùng để ký.')
      await loadKeys()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể kích hoạt khóa')
    } finally {
      setBusyId(null)
    }
  }

  const confirmRevoke = async () => {
    if (!revokeTarget || revokeReason.trim().length < 5) {
      showNotification('error', 'Vui lòng nhập lý do thu hồi rõ ràng')
      return
    }
    try {
      setBusyId(revokeTarget.id)
      await revokePQCKey(
        revokeTarget.id,
        revokeReason.trim(),
        compromisedAt ? new Date(compromisedAt).toISOString() : undefined
      )
      showNotification('success', 'Đã thu hồi khóa')
      setRevokeTarget(null)
      setRevokeReason('')
      setCompromisedAt('')
      await loadKeys()
    } catch (error: any) {
      showNotification('error', error.message || 'Không thể thu hồi khóa')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className='space-y-5'>
      <Alert variant='success'>
        <ShieldCheck />
        <AlertTitle>Khóa ký văn bằng</AlertTitle>
        <AlertDescription>
          {activeKey ? (
            <>
              Đang dùng: <strong>{activeKey.name}</strong> · {activeKey.algorithm}. Văn bằng đã ký bằng khóa cũ vẫn
              được xác minh bình thường.
            </>
          ) : (
            'Chưa có khóa nào được kích hoạt. Tạo và kích hoạt một khóa trước khi ký văn bằng.'
          )}
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-lg'>
            <KeyRound className='size-5' /> Tạo khóa ký mới
          </CardTitle>
          <CardDescription>
            Khóa bí mật được mã hóa và lưu trên máy chủ, không bao giờ rời khỏi hệ thống.
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div>
            <Label className='mb-2 block'>Thuật toán ký (FIPS 204)</Label>
            <div role='radiogroup' aria-label='Thuật toán ký' className='grid gap-3 md:grid-cols-3'>
              {PQC_ALGORITHMS.map((option) => {
                const selected = newKeyAlgorithm === option.value
                return (
                  <button
                    key={option.value}
                    type='button'
                    role='radio'
                    aria-checked={selected}
                    onClick={() => setNewKeyAlgorithm(option.value)}
                    className={cn(
                      'rounded-lg border p-3 text-left text-sm transition-colors hover:border-indigo-400',
                      selected && 'border-indigo-600 bg-indigo-50 ring-1 ring-indigo-600 dark:bg-indigo-950'
                    )}
                  >
                    <div className='flex items-center justify-between gap-2'>
                      <span className='font-semibold'>{option.value}</span>
                      {option.value === DEFAULT_PQC_ALGORITHM && <Badge>Khuyến nghị</Badge>}
                    </div>
                    <div className='mt-1 text-xs text-muted-foreground'>{option.level}</div>
                    <p className='mt-2 text-xs'>{option.description}</p>
                    <p className='mt-2 text-xs text-muted-foreground'>
                      Khóa công khai {option.publicKeySize.toLocaleString('vi-VN')} B · Chữ ký{' '}
                      {option.signatureSize.toLocaleString('vi-VN')} B
                    </p>
                  </button>
                )
              })}
            </div>
          </div>
          <div className='flex flex-col gap-3 sm:flex-row'>
            <Input
              aria-label='Tên khóa'
              value={newKeyName}
              onChange={(event) => setNewKeyName(event.target.value)}
              maxLength={100}
            />
            <Button onClick={createKey} isLoading={busyId === 'create'}>
              <Plus /> Tạo khóa {newKeyAlgorithm}
            </Button>
            <Button variant='outline' size='icon' onClick={loadKeys} isLoading={loading} title='Làm mới'>
              <RefreshCw />
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className='grid gap-4 lg:grid-cols-2'>
        {keys.map((key) => (
          <Card key={key.id} className={key.status === 'active' ? 'border-emerald-500' : ''}>
            <CardHeader>
              <div className='flex items-start justify-between gap-3'>
                <div>
                  <CardTitle className='text-base'>{key.name}</CardTitle>
                  <CardDescription>{key.algorithm}</CardDescription>
                </div>
                <Badge
                  variant={key.status === 'active' ? 'default' : key.status === 'revoked' ? 'destructive' : 'outline'}
                >
                  {statusLabel[key.status]}
                </Badge>
              </div>
            </CardHeader>
            <CardContent className='space-y-3 text-sm'>
              <div>
                <div className='text-muted-foreground'>Dấu vân tay khóa công khai</div>
                <code className='break-all text-xs'>{key.public_key_fingerprint}</code>
              </div>
              <div className='text-muted-foreground'>Tạo lúc {new Date(key.created_at).toLocaleString('vi-VN')}</div>
              {key.revocation_reason && (
                <Alert variant='warning'>
                  <AlertTriangle />
                  <AlertTitle>Lý do thu hồi</AlertTitle>
                  <AlertDescription>{key.revocation_reason}</AlertDescription>
                </Alert>
              )}
              <div className='flex gap-2'>
                {key.status === 'pending' && (
                  <Button size='sm' onClick={() => activateKey(key)} isLoading={busyId === key.id}>
                    <CheckCircle2 /> Kích hoạt
                  </Button>
                )}
                {key.status !== 'revoked' && (
                  <Button size='sm' variant='destructive' onClick={() => setRevokeTarget(key)}>
                    <ShieldX /> Thu hồi
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {!loading && keys.length === 0 && (
        <div className='rounded-md border border-dashed p-8 text-center text-muted-foreground'>
          Chưa có khóa ký nào.
        </div>
      )}

      <Dialog open={revokeTarget !== null} onOpenChange={(open) => !open && setRevokeTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Thu hồi khóa {revokeTarget?.name}</DialogTitle>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <Label>Lý do thu hồi</Label>
              <Textarea value={revokeReason} onChange={(event) => setRevokeReason(event.target.value)} />
            </div>
            <div className='space-y-2'>
              <Label>Thời điểm khóa có thể đã bị lộ (không bắt buộc)</Label>
              <Input
                type='datetime-local'
                value={compromisedAt}
                onChange={(event) => setCompromisedAt(event.target.value)}
              />
              <p className='text-xs text-muted-foreground'>
                Chỉ điền khi nghi ngờ lộ khóa. Chữ ký từ thời điểm này trở đi sẽ bị đánh dấu không hợp lệ; thu hồi do
                luân chuyển thông thường không làm mất hiệu lực chữ ký cũ.
              </p>
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setRevokeTarget(null)}>
              Hủy
            </Button>
            <Button variant='destructive' onClick={confirmRevoke} isLoading={busyId === revokeTarget?.id}>
              Thu hồi khóa
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

export default KeyManagement
