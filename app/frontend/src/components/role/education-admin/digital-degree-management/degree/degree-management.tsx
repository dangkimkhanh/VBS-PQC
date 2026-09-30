'use client'

import PageHeader from '@/components/common/page-header'
import CommonPagination from '@/components/common/pagination'
import { UseData } from '@/components/providers/data-provider'
import Filter from '@/components/common/filter'
import TableList from '@/components/common/table-list'
import { Badge } from '@/components/ui/badge'
import { CERTIFICATE_TYPE_OPTIONS, PAGE_SIZE } from '@/constants/common'
import { formatFacultyOptionsByID } from '@/lib/utils/format-api'
import { useCallback, useState } from 'react'
import useSWR from 'swr'
import {
  importDigitalDegreeExcel,
  searchDigitalDegreeList,
  uploadDigitalDegreesBlockchain,
  uploadRevocationsBlockchain
} from '@/lib/api/digital-degree'
import { formatResponseImportExcel, findLabel, showNotification } from '@/lib/utils/common'
import { Button } from '@/components/ui/button'
import { AlertCircleIcon, Blocks, CheckCircle2Icon, Eye, Grid2X2Check, Info } from 'lucide-react'
import SignDegreeButton from './sign-degree-button'
import ImportDegreeExcelDialog from './import-degree-excel-dialog'
import RevokeRoundDialog from './revoke-round-dialog'
import AssignRoundDialog from './assign-round-dialog'
import { ROUND_NONE } from '@/components/common/round-picker'
import IssueSingleDegreeDialog from './issue-single-degree-dialog'
import CreateDigitalDegreeDialog from './create-digital-degree-dialog'
import RevokeDegreeDialog from './revoke-degree-dialog'
import { DegreeActions } from '@/components/common/degree-portal'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from '@/components/ui/alert-dialog'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import Link from 'next/link'
import ExportExcelButton from '../../export-excel-button'
import useSWRMutation from 'swr/mutation'

const DegreeManagement = () => {
  const [filter, setFilter] = useState<any>({
    faculty_id: '',
    round_id: '',
    keyword: '',
    certificate_type: '',
    course: '',
    issued: 'true',
    page: 1
  })
  const facultyOptions = formatFacultyOptionsByID(UseData().facultyList)
  const queryCertificates = useSWR('digital-degree-list' + JSON.stringify(filter), () =>
    searchDigitalDegreeList({
      ...filter,
      page: filter.page || 1,
      page_size: PAGE_SIZE,
      issued: filter.issued === 'revoked' ? undefined : filter.issued === 'true',
      revoked: filter.issued === 'revoked' ? true : undefined
    })
  )

  // Anchoring and ledger revocation act on one round: one faculty of it, or all of them.
  const scopeReady = Boolean(filter.round_id && filter.round_id !== ROUND_NONE)
  const facultyText = filter.faculty_id ? findLabel(filter.faculty_id, facultyOptions) : 'Tất cả chuyên ngành'
  const revocationMode = filter.issued === 'revoked'
  const selectedRoundName = scopeReady ? queryCertificates.data?.data?.[0]?.round_name : undefined

  const mutatePushDegreesBlockchain = useSWRMutation(
    'push-digital-degree-blockchain',
    async (_key, { arg }: { arg: any }) => {
      const res =
        arg.issued === 'revoked'
          ? await uploadRevocationsBlockchain(arg.faculty_id, arg.round_id)
          : await uploadDigitalDegreesBlockchain(arg.faculty_id, arg.round_id, arg.certificate_type, arg.course)
      queryCertificates.mutate()

      return res
    },
    {
      onError: (error) => {
        showNotification('error', error.message || 'Không thể ghi lên Blockchain')
      },
      onSuccess: (res: any) => {
        if (res?.data?.recorded !== undefined) {
          const skipped = res.data.not_anchored
            ? `; ${res.data.not_anchored} văn bằng chưa từng ghi Blockchain nên không cần ghi thu hồi`
            : ''
          showNotification('success', `Đã ghi thu hồi của ${res.data.recorded} văn bằng lên Blockchain${skipped}`)
          return
        }
        showNotification('success', 'Đã ghi lên Blockchain')
      }
    }
  )

  // Errors are shown by the import dialog, which stays open so the user can fix them.
  const mutateImportExcel = useSWRMutation(
    'import-digital-degree-excel',
    (_key, { arg }: { arg: FormData }) => importDigitalDegreeExcel(arg),
    {
      onSuccess: (data) => {
        const formatData = formatResponseImportExcel(data)

        if (data.error_count === 0) {
          showNotification('success', `Đã thêm ${data.success_count} văn bằng`)
          queryCertificates.mutate()
          return
        }

        if (data.success_count === 0) {
          formatData.error.forEach((item) => {
            showNotification('error', `Dòng ${item.row.join(', ')}: ${item.title}`)
          })
          return
        }

        formatData.error.forEach((item) => {
          showNotification('error', `Dòng ${item.row.join(', ')}: ${item.title}`)
        })

        showNotification('success', `Đã thêm các dòng ${formatData.success.join(', ')}`)
        queryCertificates.mutate()
      }
    }
  )

  const handleUpload = useCallback((file: FormData) => mutateImportExcel.trigger(file), [mutateImportExcel])

  return (
    <>
      <PageHeader
        title='Quản lý văn bằng số'
        extra={[
          <ImportDegreeExcelDialog key='upload-excel' onSubmit={handleUpload} />,
          <CreateDigitalDegreeDialog key='create-digital-degree' onCreated={() => queryCertificates.mutate()} />,
          <SignDegreeButton
            key='sign-degree-button'
            filter={filter}
            facultyName={facultyText}
            roundName={selectedRoundName}
            onSigned={() => queryCertificates.mutate()}
          />,
          <AlertDialog key='blockchain-alert'>
            <AlertDialogTrigger asChild>
              <Button
                title={revocationMode ? 'Ghi các văn bằng đã thu hồi lên Blockchain' : 'Ghi lên Blockchain'}
                variant={'outline'}
                isLoading={mutatePushDegreesBlockchain.isMutating}
              >
                <Blocks />
                <span className='hidden md:block'>{revocationMode ? 'Ghi thu hồi lên Blockchain' : 'Ghi lên Blockchain'}</span>
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>{revocationMode ? 'Ghi thu hồi lên Blockchain' : 'Ghi lên Blockchain'}</AlertDialogTitle>
              </AlertDialogHeader>
              {scopeReady ? (
                <Alert variant={'success'}>
                  <CheckCircle2Icon />
                  <AlertTitle>Sẵn sàng</AlertTitle>
                  <AlertDescription>
                    <ul className='list-inside list-disc'>
                      <li>Chuyên ngành: {facultyText}</li>
                      <li>Đợt cấp: {selectedRoundName || 'đã chọn trong bộ lọc'}</li>
                      {!revocationMode && filter.certificate_type && <li>Loại bằng: {filter.certificate_type}</li>}
                      {!revocationMode && filter.course && <li>Khóa học: {filter.course}</li>}
                    </ul>
                    <p className='mt-2'>
                      {revocationMode
                        ? 'Việc thu hồi các văn bằng đã ghi Blockchain của đợt này sẽ được ghi vĩnh viễn lên sổ cái, kèm chữ ký ML-DSA của trường.'
                        : 'Các văn bằng đã cấp và ký của đợt này sẽ được gom thành một lô và ghi Merkle root lên sổ cái.'}
                    </p>
                  </AlertDescription>
                </Alert>
              ) : (
                <Alert variant={'warning'}>
                  <AlertCircleIcon />
                  <AlertTitle>Cảnh báo</AlertTitle>
                  <AlertDescription>
                    Vui lòng chọn <strong>đợt cấp</strong> trong bộ lọc trước khi ghi lên Blockchain. Chọn thêm chuyên
                    ngành nếu chỉ muốn ghi một chuyên ngành của đợt.
                  </AlertDescription>
                </Alert>
              )}
              <AlertDialogFooter>
                <AlertDialogCancel>Hủy</AlertDialogCancel>
                <AlertDialogAction
                  disabled={!scopeReady}
                  onClick={() => mutatePushDegreesBlockchain.trigger(filter)}
                >
                  Xác nhận
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>,
          <RevokeRoundDialog
            key='revoke-round'
            facultyId={filter.faculty_id}
            roundId={filter.round_id}
            roundName={selectedRoundName}
            onRevoked={() => queryCertificates.mutate()}
          />,
          <ExportExcelButton
            key='export-excel'
            fileName='van-bang-so'
            queryFn={() =>
              searchDigitalDegreeList({ ...filter, page: 1, page_size: 10000 }).then((res) =>
                res.data.map((item: any) => {
                  return {
                    ...item,
                    on_blockchain: item.on_blockchain ? 'Đã ghi' : 'Chưa ghi',
                    data_encrypted: item.pqc_proof ? `Đã ký ${item.pqc_proof.algorithm}` : 'Chưa ký'
                  }
                })
              )
            }
            mapHeader={[
              { headerName: 'Mã sinh viên', key: 'student_code' },
              { headerName: 'Họ và tên', key: 'student_name' },
              { headerName: 'Đợt cấp', key: 'round_name' },
              { headerName: 'Chuyên ngành', key: 'faculty_name' },
              { headerName: 'Tên văn bằng', key: 'name' },
              { headerName: 'Phân loại', key: 'certificate_type' },
              { headerName: 'Mẫu bằng', key: 'template_name' },
              { headerName: 'Khóa', key: 'course' },
              { headerName: 'Ngày cấp bằng', key: 'issue_date' },
              { headerName: 'Chữ ký số', key: 'data_encrypted' },
              { headerName: 'Blockchain', key: 'on_blockchain' }
            ]}
          />
        ]}
      />

      <Filter
        items={[
          {
            type: 'select',
            name: 'faculty_id',
            placeholder: 'Tất cả chuyên ngành',
            setting: {
              select: {
                groups: [
                  {
                    label: 'Chuyên ngành',
                    options: facultyOptions
                  }
                ]
              }
            }
          },
          {
            type: 'round_select',
            name: 'round_id',
            placeholder: 'Chọn đợt cấp',
            setting: { roundSelect: { allowNone: true } }
          },
          {
            type: 'input',
            name: 'keyword',
            placeholder: 'Mã SV hoặc họ tên'
          },
          {
            type: 'select',
            placeholder: 'Chọn loại bằng',
            name: 'certificate_type',
            setting: {
              select: {
                groups: [
                  {
                    label: 'Bằng tốt nghiệp',
                    options: CERTIFICATE_TYPE_OPTIONS
                  }
                ]
              }
            }
          },
          {
            type: 'input',
            name: 'course',
            placeholder: 'Nhập khóa học'
          },
          {
            type: 'select',
            name: 'issued',
            placeholder: 'Chọn trạng thái cấp',
            defaultValue: 'true',
            setting: {
              select: {
                groups: [
                  {
                    label: 'Trạng thái',
                    options: [
                      { label: 'Đã cấp', value: 'true' },
                      { label: 'Chưa cấp', value: 'false' },
                      { label: 'Đã thu hồi', value: 'revoked' }
                    ]
                  }
                ]
              }
            }
          },
        ]}
        handleSetFilter={setFilter}
      />
      {filter.round_id === ROUND_NONE && (
        <Alert className='mt-4' variant='warning'>
          <Info />
          <AlertTitle>Hồ sơ chưa gắn đợt cấp</AlertTitle>
          <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
            <span>
              Các hồ sơ này được tạo trước khi có đợt cấp. Gán chúng vào một đợt để cấp, ký, ghi Blockchain và thu hồi
              theo đợt.
            </span>
            <AssignRoundDialog
              facultyId={filter.faculty_id}
              facultyLabel={facultyText}
              course={filter.course}
              certificateType={filter.certificate_type}
              onAssigned={() => queryCertificates.mutate()}
            />
          </AlertDescription>
        </Alert>
      )}
      <TableList
        items={[
          { header: 'Mã SV', value: 'student_code', className: 'min-w-[80px] font-semibold text-blue-500' },
          { header: 'Họ và tên', value: 'student_name', className: 'min-w-[150px]' },
          {
            header: 'Đợt cấp',
            value: 'round_name',
            className: 'min-w-[150px]',
            render: (item) => item.round_name || <span className='text-muted-foreground'>Chưa gắn đợt</span>
          },
          { header: 'Chuyên ngành', value: 'faculty_name', className: 'min-w-[150px]' },
          { header: 'Tên văn bằng', value: 'name', className: 'min-w-[200px]' },
          {
            header: 'Phân loại',
            value: 'isDegree',
            render: (item) => (
              <Badge className='bg-blue-500 text-white hover:bg-blue-400'>{item.certificate_type}</Badge>
            )
          },
          { header: 'Mẫu bằng', value: 'template_name', className: 'min-w-[150px]' },
          { header: 'Khóa', value: 'course' },
          {
            header: 'Ngày cấp bằng',
            value: 'issue_date',
            className: 'min-w-[100px]'
          },
          {
            header: 'Chữ ký số',
            value: 'data_encrypted',
            className: 'min-w-[150px]',
            render: (item) => (
              <Badge variant={item.pqc_proof ? 'default' : 'outline'}>{item.pqc_proof ? item.pqc_proof.algorithm : 'Chưa ký'}</Badge>
            )
          },
          {
            header: 'Blockchain',
            value: 'on_blockchain',

            render: (item) => (
              <div className='flex flex-wrap gap-1'>
                <Badge variant={item.on_blockchain ? 'default' : 'outline'}>
                  {item.on_blockchain ? 'Đã ghi' : 'Chưa ghi'}
                </Badge>
                {item.revocation_on_chain && <Badge variant='destructive'>Thu hồi đã ghi</Badge>}
              </div>
            )
          },

          {
            header: 'Hành động',
            className: 'min-w-[260px]',
            value: 'action',
            render: (item) => (
              <div className='flex flex-wrap items-center gap-2'>
                <DegreeActions degree={item} onChange={() => queryCertificates.mutate()} />
                {item.revoked && <Badge variant='destructive'>Đã thu hồi</Badge>}
                {!item.issued && item.faculty_id && (
                  <IssueSingleDegreeDialog
                    diplomaId={item.id}
                    facultyId={item.faculty_id}
                    studentName={item.student_name}
                    onIssued={() => queryCertificates.mutate()}
                  />
                )}
                {item.issued && !item.revoked && (
                  <RevokeDegreeDialog diplomaId={item.id} onRevoked={() => queryCertificates.mutate()} />
                )}
                <Link href={`/education-admin/digital-degree-management/${item.id}`}>
                  <Button size={'icon'} variant={'outline'} title='Xem chi tiết văn bằng'>
                    <Eye />
                  </Button>
                </Link>
                {item.issued && (
                  <Link href={`/verify/${item.id}`} target='_blank'>
                    <Button size={'icon'} title='Xác minh công khai (chữ ký, PDF, Blockchain)'>
                      <Grid2X2Check />
                    </Button>
                  </Link>
                )}
              </div>
            )
          }
        ]}
        data={queryCertificates.data?.data || []}
        page={queryCertificates.data?.page || 1}
        pageSize={queryCertificates.data?.page_size || PAGE_SIZE}
      />
      <CommonPagination
        page={queryCertificates.data?.page || 1}
        totalPage={queryCertificates.data?.total_page || 1}
        handleChangePage={(page) => {
          setFilter({ ...filter, page })
        }}
      />
    </>
  )
}

export default DegreeManagement
