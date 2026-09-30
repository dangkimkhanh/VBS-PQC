'use client'

import { Button } from '@/components/ui/button'
import { showNotification } from '@/lib/utils/common'
import { DownloadIcon } from 'lucide-react'
import useSWRMutation from 'swr/mutation'
import * as XLSX from 'xlsx'

interface MapHeader {
  headerName: string
  key: string
}

interface ExportExcelButtonProps {
  fileName: string
  queryFn: () => Promise<any[]>
  mapHeader: MapHeader[]
}

const ExportExcelButton: React.FC<ExportExcelButtonProps> = ({ fileName, queryFn, mapHeader }) => {
  // Hàm tính chiều rộng cột dựa trên nội dung
  const calculateColumnWidths = (data: any[], headers: MapHeader[]) => {
    const columnWidths: { [key: string]: number } = {}

    // Khởi tạo độ rộng ban đầu với độ dài của header
    headers.forEach((header) => {
      columnWidths[header.key] = header.headerName.length
    })

    // Tính độ rộng dựa trên dữ liệu
    data.forEach((row) => {
      headers.forEach((header) => {
        const cellValue = row[header.key]
        const cellLength = cellValue ? String(cellValue).length : 0
        if (cellLength > columnWidths[header.key]) {
          columnWidths[header.key] = cellLength
        }
      })
    })

    // Chuyển đổi sang format mà xlsx yêu cầu và thêm padding
    return headers.map((header) => ({
      wch: Math.min(Math.max(columnWidths[header.key] + 2, 10), 50) // Min: 10, Max: 50, padding: 2
    }))
  }

  const mutateExportExcel = useSWRMutation(
    'export-excel',
    async () => {
      // Gọi API để lấy dữ liệu
      const data = await queryFn()

      if (!data || data.length === 0) {
        throw new Error('Không có dữ liệu để xuất')
      }

      // Tạo data cho worksheet với header
      const wsData = [
        // Header row
        mapHeader.map((h) => h.headerName),
        // Data rows
        ...data.map((row) => mapHeader.map((h) => row[h.key] ?? ''))
      ]

      // Tạo worksheet
      const ws = XLSX.utils.aoa_to_sheet(wsData)

      // Tính và áp dụng độ rộng cột
      ws['!cols'] = calculateColumnWidths(data, mapHeader)

      // Tạo workbook
      const wb = XLSX.utils.book_new()
      XLSX.utils.book_append_sheet(wb, ws, 'Sheet1')

      // Xuất file
      XLSX.writeFile(wb, `${fileName}.xlsx`)

      return true
    },
    {
      onSuccess: () => {
        showNotification('success', 'Xuất file Excel thành công')
      },
      onError: (error: any) => {
        console.error('Error exporting to Excel:', error)
        showNotification('error', error.message || 'Lỗi khi xuất file Excel')
      }
    }
  )

  const handleExport = () => {
    mutateExportExcel.trigger()
  }

  return (
    <Button onClick={handleExport} isLoading={mutateExportExcel.isMutating} variant='secondary'>
      <DownloadIcon />
      <span className='hidden md:block'>Xuất Excel</span>
    </Button>
  )
}

export default ExportExcelButton
