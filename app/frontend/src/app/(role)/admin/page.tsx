'use client'

import Link from 'next/link'
import useSWR from 'swr'
import { ArrowUpRight, BadgeCheck, FileCheck2, History, School, Users } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { getAdminAudit, getAdminOverview } from '@/lib/api/university'
import { DEFAULT_PQC_ALGORITHM, PQC_ALGORITHMS } from '@/lib/api/pqc'
import { panelClass as panel, StatTile } from '@/components/common/stat-tile'
import ResendActivationButton from '@/components/role/admin/resend-activation-button'

const number = (value?: number) => (value ?? 0).toLocaleString('vi-VN')
const percent = (part?: number, total?: number) => (total ? `${Math.round((100 * (part ?? 0)) / total)}%` : '—')

const ACTION_LABELS: [RegExp, string][] = [
  [/\/lock$/, 'Khóa trường'],
  [/\/unlock$/, 'Mở khóa trường'],
  [/\/resend-activation$/, 'Gửi lại link kích hoạt'],
  [/\/universities\/:id$/, 'Cập nhật thông tin trường'],
  [/\/universities$/, 'Tạo trường']
]
const actionLabel = (action: string) => ACTION_LABELS.find(([pattern]) => pattern.test(action))?.[1] ?? action

const AdminOverviewPage = () => {
  const overview = useSWR('admin-overview', getAdminOverview)
  const audit = useSWR('admin-audit', getAdminAudit)
  const o = overview.data

  if (overview.error) {
    return (
      <p
        role='alert'
        className='rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300'
      >
        Không thể tải thống kê hệ thống. Vui lòng tải lại trang.
      </p>
    )
  }

  return (
    <div className='space-y-6'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <h2>Tổng quan</h2>
        <Link href='/admin/education-management'>
          <Button variant='outline'>
            Quản lý trường <ArrowUpRight />
          </Button>
        </Link>
      </div>

      <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        <StatTile
          icon={<School className='size-4' />}
          label='Trường'
          value={overview.isLoading ? '…' : number(o?.universities.total)}
        >
          <Link className='block hover:underline' href='/admin/education-management?status=active'>
            {number(o?.universities.active)} đang hoạt động
          </Link>
          <Link className='block hover:underline' href='/admin/education-management?status=pending'>
            {number(o?.universities.admin_pending)} chờ kích hoạt
          </Link>
          <Link className='block hover:underline' href='/admin/education-management?status=locked'>
            {number(o?.universities.locked)} đã khóa
          </Link>
        </StatTile>
        <StatTile
          icon={<Users className='size-4' />}
          label='Hồ sơ sinh viên'
          value={overview.isLoading ? '…' : number(o?.students.total)}
        >
          <p>
            {number(o?.students.activated)} đã kích hoạt tài khoản ({percent(o?.students.activated, o?.students.total)})
          </p>
        </StatTile>
        <StatTile
          icon={<FileCheck2 className='size-4' />}
          label='Văn bằng đã cấp'
          value={overview.isLoading ? '…' : number(o?.diplomas.issued)}
        >
          <p>{number(o?.diplomas.signed)} đã ký ML-DSA</p>
          <p>{number(o?.diplomas.anchored)} đã ghi Blockchain</p>
          <p>{number(o?.diplomas.revoked)} đã thu hồi</p>
        </StatTile>
        <StatTile
          icon={<BadgeCheck className='size-4' />}
          label='Lượt xác minh công khai'
          value={overview.isLoading ? '…' : number(o?.verifications.attempts)}
        >
          <p>
            {number(o?.verifications.passed)} xác minh đầy đủ thành công (
            {percent(o?.verifications.passed, o?.verifications.attempts)})
          </p>
        </StatTile>
      </div>

      <div className='grid gap-4 lg:grid-cols-3'>
        <section className={panel}>
          <h3 className='mb-3 font-semibold'>Thuật toán ký đang dùng</h3>
          <ul className='divide-y text-sm [&_p]:text-sm'>
            {PQC_ALGORITHMS.map((algorithm) => (
              <li key={algorithm.value} className='flex items-center justify-between py-2'>
                <span>
                  {algorithm.value} <span className='text-muted-foreground'>· {algorithm.level}</span>
                  {algorithm.value === DEFAULT_PQC_ALGORITHM && (
                    <span className='ml-2 text-xs text-muted-foreground'>(khuyến nghị)</span>
                  )}
                </span>
                <span className='font-semibold'>{number(o?.algorithms?.[algorithm.value])}</span>
              </li>
            ))}
          </ul>
        </section>

        <section className={panel + ' lg:col-span-2'}>
          <div className='mb-3 flex items-center justify-between'>
            <h3 className='font-semibold'>Chờ kích hoạt</h3>
            <Link className='text-sm text-main hover:underline' href='/admin/education-management?status=pending'>
              Xem tất cả
            </Link>
          </div>
          {!o?.pending_activations?.length ? (
            <p className='py-4 text-sm text-muted-foreground'>Không có trường nào đang chờ kích hoạt.</p>
          ) : (
            <ul className='divide-y text-sm [&_p]:text-sm'>
              {o.pending_activations.map((u: any) => (
                <li key={u.id} className='flex flex-wrap items-center justify-between gap-2 py-2'>
                  <div>
                    <p className='font-medium'>
                      {u.university_code} · {u.university_name}
                    </p>
                    <p className='text-muted-foreground'>
                      {u.admin_email} · tạo lúc {u.created_at}
                    </p>
                  </div>
                  <ResendActivationButton
                    id={u.id}
                    resendAt={u.activation_resend_at}
                    onSent={() => overview.mutate()}
                  />
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      <div className='grid gap-4 lg:grid-cols-2'>
        <section className={panel}>
          <h3 className='mb-3 font-semibold'>Trường cấp nhiều văn bằng nhất</h3>
          {!o?.top_universities?.length ? (
            <p className='py-4 text-sm text-muted-foreground'>Chưa có văn bằng nào được cấp.</p>
          ) : (
            <table className='w-full text-sm'>
              <thead className='text-left text-muted-foreground'>
                <tr>
                  <th className='py-2 font-medium'>Trường</th>
                  <th className='py-2 text-right font-medium'>Đã cấp</th>
                  <th className='py-2 text-right font-medium'>Thu hồi</th>
                  <th className='py-2 text-right font-medium'>SV kích hoạt</th>
                </tr>
              </thead>
              <tbody className='divide-y'>
                {o.top_universities.map((u: any) => (
                  <tr key={u.id}>
                    <td className='py-2'>
                      <Link
                        className='hover:underline'
                        href={`/admin/education-management?q=${encodeURIComponent(u.university_code)}`}
                      >
                        {u.university_code} · {u.university_name}
                      </Link>
                    </td>
                    <td className='py-2 text-right tabular-nums'>{number(u.stats?.issued)}</td>
                    <td className='py-2 text-right tabular-nums'>{number(u.stats?.revoked)}</td>
                    <td className='py-2 text-right tabular-nums'>
                      {number(u.stats?.activated_students)}/{number(u.stats?.students)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>

        <section className={panel}>
          <h3 className='mb-3 flex items-center gap-2 font-semibold'>
            <History className='size-4 text-main' /> Hoạt động quản trị gần đây
          </h3>
          {audit.error ? (
            <p role='alert' className='text-sm text-red-600 dark:text-red-400'>
              Không thể tải nhật ký.
            </p>
          ) : !audit.data?.length ? (
            <p className='py-4 text-sm text-muted-foreground'>Chưa có thao tác nào được ghi nhận.</p>
          ) : (
            <ol className='max-h-80 divide-y overflow-y-auto text-sm [&_p]:text-sm'>
              {audit.data.map((e: any, i: number) => (
                <li key={i} className='flex items-start justify-between gap-3 py-2'>
                  <div>
                    <p>
                      {actionLabel(e.action)}{' '}
                      <span className={e.success ? 'text-muted-foreground' : 'text-red-600 dark:text-red-400'}>
                        · {e.success ? 'thành công' : 'không thành công'}
                      </span>
                    </p>
                    <p className='text-muted-foreground'>
                      {e.university_code ? `${e.university_code} · ${e.university_name}` : e.university_id}
                    </p>
                  </div>
                  <time className='shrink-0 text-xs text-muted-foreground'>{e.at}</time>
                </li>
              ))}
            </ol>
          )}
        </section>
      </div>
    </div>
  )
}

export default AdminOverviewPage
