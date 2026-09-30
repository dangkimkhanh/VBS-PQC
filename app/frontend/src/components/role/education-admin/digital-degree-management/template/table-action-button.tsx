'use client'

import { Button } from '@/components/ui/button'
import { CodeXml, PencilIcon } from 'lucide-react'
import { Dispatch, SetStateAction } from 'react'
import Link from 'next/link'

interface Props {
  id: string
  handleSetIdDetail: Dispatch<SetStateAction<string | null | undefined>>
  canEdit: boolean
  templateSampleId: string
}

// Templates are not signed: each diploma is signed individually with ML-DSA when issued.
const TableActionButton: React.FC<Props> = (props) => {
  return (
    <div className='flex gap-2'>
      <Button
        variant='outline'
        size='icon'
        title={props.canEdit ? 'Sửa mẫu bằng' : 'Mẫu đã dùng để cấp bằng, không thể sửa'}
        onClick={() => props.handleSetIdDetail(props.id)}
        disabled={!props.canEdit}
      >
        <PencilIcon />
      </Button>
      <Link
        href={`/education-admin/digital-degree-management?tab=template-interface&id=${props.templateSampleId}`}
        target='_blank'
      >
        <Button size='icon' title='Xem giao diện mẫu'>
          <CodeXml />
        </Button>
      </Link>
    </div>
  )
}

export default TableActionButton
