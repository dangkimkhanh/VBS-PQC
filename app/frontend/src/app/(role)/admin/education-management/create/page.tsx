'use client'

import { useRouter } from 'next/navigation'
import PageHeader from '@/components/common/page-header'
import UniversityForm from '@/components/role/admin/university-form'
import { createUniversity } from '@/lib/api/university'
import { showNotification } from '@/lib/utils/common'

export default function CreateUniversityPage() {
  const router = useRouter()
  return (
    <>
      <PageHeader title='Tạo trường' />
      <UniversityForm
        mode='create'
        submitLabel='Tạo trường'
        onSubmit={async (payload) => {
          try {
            const res = await createUniversity(payload)
            showNotification(res.email_sent ? 'success' : 'warning', res.message)
            router.push('/admin/education-management')
          } catch (error: any) {
            showNotification('error', error.message || 'Không thể tạo trường')
            throw error
          }
        }}
      />
    </>
  )
}
