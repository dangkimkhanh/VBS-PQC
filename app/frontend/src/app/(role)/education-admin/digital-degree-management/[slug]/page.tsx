import Back from '@/components/common/back'
import DigitalDegreeView from '@/components/common/digital-degree-view'
import { DegreeHistory } from '@/components/common/degree-portal'

interface Props {
  params: Promise<{ slug: string }>
}

const DigitalDegreeDetailPage = async ({ params }: Props) => {
  const { slug } = await params

  return (
    <>
      <div className='mb-4 flex items-center gap-2'>
        <Back />
        <h2>Chi tiết văn bằng số</h2>
      </div>
      <DigitalDegreeView id={slug} />
      <DegreeHistory id={slug} />
    </>
  )
}

export default DigitalDegreeDetailPage
