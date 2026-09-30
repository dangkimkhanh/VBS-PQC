'use client'
import Link from 'next/link'
import useSWR from 'swr'
import { Suspense, useEffect, useState } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'
import { Lock, LockOpen, PackagePlus, PencilIcon, Search } from 'lucide-react'
import PageHeader from '@/components/common/page-header'
import TableList from '@/components/common/table-list'
import CommonPagination from '@/components/common/pagination'
import SuspendPage from '@/components/common/suspend-page'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle
} from '@/components/ui/alert-dialog'
import { getUniversityList, lockUniversity, unlockUniversity, UniversityStatusFilter } from '@/lib/api/university'
import { showNotification } from '@/lib/utils/common'
import ResendActivationButton from '@/components/role/admin/resend-activation-button'

const PAGE_SIZE = 20

const ACCOUNT_STATUS: Record<string, { label: string; variant: 'default' | 'outline' | 'destructive' | 'secondary' }> =
  {
    active: { label: 'Đã kích hoạt', variant: 'default' },
    pending: { label: 'Chờ kích hoạt', variant: 'outline' },
    locked: { label: 'Đã khóa', variant: 'destructive' }
  }

const STATUS_FILTERS: { value: UniversityStatusFilter; label: string }[] = [
  { value: '', label: 'Tất cả' },
  { value: 'active', label: 'Đang hoạt động' },
  { value: 'pending', label: 'Chờ kích hoạt' },
  { value: 'locked', label: 'Đã khóa' }
]

type PendingAction = { kind: 'lock' | 'unlock'; id: string; name: string } | null

const EducationManagement = () => {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const q = searchParams.get('q') ?? ''
  const status = (searchParams.get('status') ?? '') as UniversityStatusFilter
  const page = Math.max(1, Number(searchParams.get('page')) || 1)

  const [keyword, setKeyword] = useState(q)
  const [pending, setPending] = useState<PendingAction>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  // Filters live in the URL so they survive reloads and can be linked from the overview.
  const setParams = (next: { q?: string; status?: string; page?: number }) => {
    const params = new URLSearchParams(searchParams.toString())
    Object.entries(next).forEach(([key, value]) => {
      if (value === undefined || value === '' || (key === 'page' && value === 1)) params.delete(key)
      else params.set(key, String(value))
    })
    router.replace(params.toString() ? `${pathname}?${params}` : pathname)
  }

  useEffect(() => {
    if (keyword.trim() === q) return
    const timer = setTimeout(() => setParams({ q: keyword.trim(), page: 1 }), 400)
    return () => clearTimeout(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keyword])

  const query = useSWR(['university-list', q, status, page], () =>
    getUniversityList({ q, status, page, page_size: PAGE_SIZE })
  )

  const run = async (id: string, action: () => Promise<any>, fallback: string) => {
    setBusyId(id)
    try {
      const res = await action()
      showNotification('success', res?.message || 'Thao tác thành công')
      query.mutate()
    } catch (error: any) {
      showNotification('error', error.message || fallback)
    } finally {
      setBusyId(null)
    }
  }

  return (
    <>
      <PageHeader
        title='Quản lý trường'
        extra={[
          <Link key='create' href='/admin/education-management/create'>
            <Button>
              <PackagePlus /> Tạo trường
            </Button>
          </Link>
        ]}
      />
      <div className='flex flex-col gap-3 md:flex-row md:items-center'>
        <div className='relative md:w-96'>
          <Search className='absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground' />
          <Input
            aria-label='Tìm trường'
            className='pl-9'
            placeholder='Tìm theo tên, mã trường, tên miền, email quản trị…'
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
          />
        </div>
        <div role='tablist' aria-label='Lọc theo trạng thái' className='flex flex-wrap gap-2'>
          {STATUS_FILTERS.map((option) => (
            <Button
              key={option.value || 'all'}
              role='tab'
              aria-selected={status === option.value}
              size='sm'
              variant={status === option.value ? 'default' : 'outline'}
              onClick={() => setParams({ status: option.value, page: 1 })}
            >
              {option.label}
            </Button>
          ))}
        </div>
        <span className='text-sm text-muted-foreground md:ml-auto'>
          {query.data ? `${query.data.total} trường` : query.isLoading ? 'Đang tải…' : ''}
        </span>
      </div>

      <TableList
        items={[
          { header: 'Mã trường', value: 'university_code', className: 'font-semibold text-blue-500 min-w-[90px]' },
          {
            header: 'Tên trường',
            value: 'university_name',
            className: 'min-w-[200px]',
            render: (item) => (
              <div>
                <p>{item.university_name}</p>
                <p className='text-xs text-muted-foreground'>@{item.email_domain}</p>
              </div>
            )
          },
          {
            header: 'Quản trị trường',
            value: 'admin_email',
            render: (item) => (
              <div className='text-sm'>
                <p>{item.admin_name}</p>
                <p className='text-muted-foreground'>{item.admin_email}</p>
              </div>
            )
          },
          {
            header: 'Sinh viên',
            value: 'students',
            render: (item) => (
              <span title='Đã kích hoạt / tổng hồ sơ'>
                {item.stats?.activated_students ?? 0}/{item.stats?.students ?? 0}
              </span>
            )
          },
          {
            header: 'Văn bằng',
            value: 'diplomas',
            render: (item) => (
              <div className='text-sm'>
                <p>{item.stats?.issued ?? 0} đã cấp</p>
                {item.stats?.revoked > 0 && <p className='text-destructive'>{item.stats.revoked} thu hồi</p>}
              </div>
            )
          },
          {
            header: 'Tài khoản',
            value: 'admin_account_status',
            render: (item) => {
              const s = item.status === 'locked' ? ACCOUNT_STATUS.locked : ACCOUNT_STATUS[item.admin_account_status]
              return s ? <Badge variant={s.variant}>{s.label}</Badge> : <Badge variant='secondary'>Chưa có</Badge>
            }
          },
          {
            header: 'Hành động',
            value: 'action',
            render: (item) => (
              <div className='flex items-center gap-2'>
                <Link href={`/admin/education-management/${item.id}`}>
                  <Button size='icon' variant='outline' title='Cập nhật thông tin'>
                    <PencilIcon />
                  </Button>
                </Link>
                {item.status !== 'locked' && item.admin_account_status === 'pending' && (
                  <ResendActivationButton
                    compact
                    id={item.id}
                    resendAt={item.activation_resend_at}
                    onSent={() => query.mutate()}
                  />
                )}
                {item.status === 'locked' ? (
                  <Button
                    size='icon'
                    variant='secondary'
                    title='Mở khóa trường'
                    disabled={busyId === item.id}
                    onClick={() => setPending({ kind: 'unlock', id: item.id, name: item.university_name })}
                  >
                    <LockOpen />
                  </Button>
                ) : (
                  <Button
                    size='icon'
                    variant='destructive'
                    title='Khóa trường'
                    disabled={busyId === item.id}
                    onClick={() => setPending({ kind: 'lock', id: item.id, name: item.university_name })}
                  >
                    <Lock />
                  </Button>
                )}
              </div>
            )
          }
        ]}
        data={query.data?.data ?? []}
        page={page}
        pageSize={PAGE_SIZE}
      />
      {query.data && query.data.total === 0 && (
        <p className='py-8 text-center text-sm text-muted-foreground'>Không có trường nào phù hợp.</p>
      )}
      {(query.data?.total_page ?? 1) > 1 && (
        <CommonPagination
          page={page}
          totalPage={query.data?.total_page ?? 1}
          handleChangePage={(next) => setParams({ page: next })}
        />
      )}

      <AlertDialog open={pending !== null} onOpenChange={(open) => !open && setPending(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{pending?.kind === 'lock' ? 'Khóa trường' : 'Mở khóa trường'}</AlertDialogTitle>
            <AlertDialogDescription>
              {pending?.kind === 'lock'
                ? `Khóa ${pending?.name}? Tài khoản quản trị của trường sẽ bị đăng xuất.`
                : `Mở khóa ${pending?.name}?`}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Hủy</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (!pending) return
                const { kind, id } = pending
                setPending(null)
                run(id, () => (kind === 'lock' ? lockUniversity(id) : unlockUniversity(id)), 'Thao tác thất bại')
              }}
            >
              Xác nhận
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

const EducationManagementPage = () => (
  <Suspense fallback={<SuspendPage />}>
    <EducationManagement />
  </Suspense>
)

export default EducationManagementPage
