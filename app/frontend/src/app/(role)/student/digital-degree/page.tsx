'use client'

import DigitalDegreeView from '@/components/common/digital-degree-view'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getDigitalDegreesByStudent } from '@/lib/api/digital-degree'
import { Suspense, useState } from 'react'
import useSWR from 'swr'
import { useRouter, useSearchParams } from 'next/navigation'
import SuspendPage from '@/components/common/suspend-page'
import { GraduationCap } from 'lucide-react'

const StudentDigitalDegree = () => {
  const [tab, setTab] = useState<string | undefined>(undefined)
  const router = useRouter()
  const searchParams = useSearchParams()
  const query = useSWR('student-digital-degrees', getDigitalDegreesByStudent, {
    onSuccess: (data) => setTab(searchParams.get('tab') ?? data?.[0]?.id)
  })
  const degrees: any[] = query.data ?? []

  return (
    <>
      <h2>Văn bằng số của tôi</h2>
      <p className='mt-1 text-sm text-muted-foreground'>
        Thông tin chỉ để xem. Nếu có sai sót, vui lòng liên hệ phòng đào tạo của trường.
      </p>
      {query.isLoading ? (
        <p className='mt-6'>Đang tải…</p>
      ) : degrees.length === 0 ? (
        <div className='mt-10 flex flex-col items-center gap-3 text-center text-muted-foreground'>
          <GraduationCap className='size-10' />
          <p>Bạn chưa có văn bằng số nào được cấp.</p>
        </div>
      ) : (
        <Tabs
          className='mt-4'
          value={tab}
          onValueChange={(value) => {
            setTab(value)
            router.replace(`/student/digital-degree?tab=${value}`)
          }}
        >
          <TabsList>
            {degrees.map((item) => (
              <TabsTrigger key={item.id} value={item.id}>
                {item.name}
                {item.revoked ? ' (đã thu hồi)' : ''}
              </TabsTrigger>
            ))}
          </TabsList>
          {degrees.map((item) => (
            <TabsContent key={item.id} value={item.id}>
              {tab === item.id && <DigitalDegreeView id={item.id} />}
            </TabsContent>
          ))}
        </Tabs>
      )}
    </>
  )
}

const StudentDigitalDegreePage = () => (
  <Suspense fallback={<SuspendPage />}>
    <StudentDigitalDegree />
  </Suspense>
)

export default StudentDigitalDegreePage
