'use client'

import { use } from 'react'
import useSWR from 'swr'
import { useRouter } from 'next/navigation'
import PageHeader from '@/components/common/page-header'
import UniversityForm from '@/components/role/admin/university-form'
import { getUniversity, updateUniversity } from '@/lib/api/university'
import { showNotification } from '@/lib/utils/common'

const EDITABLE = [
  'university_name',
  'university_code',
  'address',
  'website',
  'email_domain',
  'admin_email',
  'admin_name',
  'admin_phone',
  'signer_name',
  'verification_note',
  'description'
]

export default function EditUniversityPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const query = useSWR(`university-${id}`, () => getUniversity(id), { revalidateOnFocus: false })

  if (query.error) return <p className='p-4 text-destructive'>Không tải được thông tin trường.</p>
  if (!query.data) return <p className='p-4'>Đang tải…</p>

  const initial = Object.fromEntries(EDITABLE.map((key) => [key, query.data[key] ?? '']))

  return (
    <>
      <PageHeader title={`Cập nhật: ${query.data.university_name}`} />
      <UniversityForm
        key={id}
        mode='update'
        initial={initial}
        submitLabel='Lưu thay đổi'
        onSubmit={async (payload) => {
          if (
            payload.admin_email.trim().toLowerCase() !== String(query.data.admin_email ?? '').toLowerCase() &&
            !window.confirm(
              'Đổi email quản trị sẽ đăng xuất và vô hiệu mật khẩu của người phụ trách hiện tại, đồng thời gửi link kích hoạt tới email mới. Tiếp tục?'
            )
          ) {
            return
          }
          try {
            const res = await updateUniversity(id, payload)
            showNotification('success', res.message)
            router.push('/admin/education-management')
          } catch (error: any) {
            showNotification('error', error.message || 'Không thể cập nhật trường')
            throw error
          }
        }}
      />
    </>
  )
}
