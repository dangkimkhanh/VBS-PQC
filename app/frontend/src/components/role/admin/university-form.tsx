'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { UniversityPayload } from '@/lib/api/university'

type Field = { key: string; label: string; type?: string; placeholder?: string; required?: boolean; hint?: string }

const FIELDS: Field[] = [
  { key: 'university_name', label: 'Tên trường', required: true },
  { key: 'university_code', label: 'Mã trường', required: true },
  { key: 'address', label: 'Địa chỉ', required: true },
  { key: 'website', label: 'Website chính thức', type: 'url', placeholder: 'https://' },
  {
    key: 'email_domain',
    label: 'Tên miền email sinh viên',
    required: true,
    placeholder: 'actvn.edu.vn',
    hint: 'Phần sau @ trong email sinh viên.'
  },
  {
    key: 'admin_email',
    label: 'Email quản trị trường',
    type: 'email',
    required: true,
    hint: 'Nhận link kích hoạt tài khoản.'
  },
  { key: 'admin_name', label: 'Họ tên người phụ trách', required: true },
  { key: 'admin_phone', label: 'Số điện thoại', type: 'tel', required: true },
  {
    key: 'signer_name',
    label: 'Người ký văn bằng',
    placeholder: 'VD: Nguyễn Văn A',
    hint: 'In dưới chức danh Hiệu trưởng hoặc Giám đốc trên văn bằng.'
  }
]

interface Props {
  mode: 'create' | 'update'
  initial?: UniversityPayload
  submitLabel: string
  // eslint-disable-next-line no-unused-vars
  onSubmit: (payload: UniversityPayload) => Promise<void>
}

const UniversityForm: React.FC<Props> = ({ mode, initial, submitLabel, onSubmit }) => {
  const router = useRouter()
  const [form, setForm] = useState<UniversityPayload>(initial ?? {})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const update = (key: string, value: string) => setForm((current) => ({ ...current, [key]: value }))

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    setLoading(true)
    setErrors({})
    try {
      const payload = { ...form }
      if (mode === 'update') delete payload.university_code
      await onSubmit(payload)
    } catch (error: any) {
      // Field-level validation errors come back as { errors: { GoFieldName: message } }.
      const fieldErrors: Record<string, string> = error?.data?.errors ?? {}
      setErrors(
        Object.fromEntries(
          Object.entries(fieldErrors).map(([field, message]) => [
            field.replace(/([a-z])([A-Z])/g, '$1_$2').toLowerCase(),
            message
          ])
        )
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={submit} className='mx-auto max-w-3xl space-y-4 p-4'>
      <div className='grid gap-4 md:grid-cols-2'>
        {FIELDS.map((field) => (
          <label key={field.key} className='space-y-1 text-sm'>
            <span>
              {field.label}
              {field.required && <span className='text-destructive'> *</span>}
            </span>
            <Input
              type={field.type ?? 'text'}
              required={field.required}
              placeholder={field.placeholder}
              disabled={mode === 'update' && field.key === 'university_code'}
              value={form[field.key] || ''}
              onChange={(e) => update(field.key, e.target.value)}
            />
            {errors[field.key] ? (
              <span className='block text-xs text-destructive'>{errors[field.key]}</span>
            ) : (
              field.hint && <span className='block text-xs text-muted-foreground'>{field.hint}</span>
            )}
          </label>
        ))}
      </div>
      <label className='block space-y-1 text-sm'>
        <span>Ghi chú xác minh</span>
        <Textarea value={form.verification_note || ''} onChange={(e) => update('verification_note', e.target.value)} />
      </label>
      <label className='block space-y-1 text-sm'>
        <span>Mô tả</span>
        <Textarea value={form.description || ''} onChange={(e) => update('description', e.target.value)} />
      </label>
      <div className='flex gap-3'>
        <Button type='submit' isLoading={loading}>
          {submitLabel}
        </Button>
        <Button type='button' variant='outline' onClick={() => router.push('/admin/education-management')}>
          Hủy
        </Button>
      </div>
    </form>
  )
}

export default UniversityForm
