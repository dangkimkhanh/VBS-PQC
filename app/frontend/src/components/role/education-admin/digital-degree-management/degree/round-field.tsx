'use client'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import RoundPicker from '@/components/common/round-picker'
import { createIssuanceRound } from '@/lib/api/digital-degree'
import { cn } from '@/lib/utils/common'

export type RoundChoice = {
  mode: 'existing' | 'new'
  roundId: string
  roundLabel?: string
  name: string
  decisionNumber: string
  decisionDate: string
}

export const emptyRoundChoice: RoundChoice = { mode: 'existing', roundId: '', name: '', decisionNumber: '', decisionDate: '' }

// Returns an error to show, or '' when the choice is complete.
export const roundChoiceError = (choice: RoundChoice) => {
  if (choice.mode === 'existing') return choice.roundId ? '' : 'Vui lòng chọn đợt cấp'
  const length = choice.name.trim().length
  return length >= 3 && length <= 150 ? '' : 'Tên đợt cấp cần từ 3 đến 150 ký tự'
}

// Resolves the choice to a round ID, creating the round when needed.
export const resolveRoundChoice = async (choice: RoundChoice): Promise<string> => {
  if (choice.mode === 'existing') return choice.roundId
  const round = await createIssuanceRound({
    name: choice.name.trim(),
    decision_number: choice.decisionNumber.trim() || undefined,
    decision_date: choice.decisionDate || undefined
  })
  return round.id
}

interface Props {
  value: RoundChoice
  // eslint-disable-next-line no-unused-vars
  onChange: (value: RoundChoice) => void
  disabled?: boolean
}

const RoundField: React.FC<Props> = ({ value, onChange, disabled }) => {
  const set = (patch: Partial<RoundChoice>) => onChange({ ...value, ...patch })
  const tab = (mode: RoundChoice['mode'], text: string) => (
    <button
      type='button'
      disabled={disabled}
      onClick={() => set({ mode })}
      className={cn(
        'flex-1 rounded-md px-3 py-1.5 text-sm transition-colors',
        value.mode === mode ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground'
      )}
    >
      {text}
    </button>
  )

  return (
    <div className='space-y-3'>
      <Label>Đợt cấp*</Label>
      <div className='flex rounded-lg bg-muted p-1'>
        {tab('existing', 'Đợt có sẵn')}
        {tab('new', 'Tạo đợt mới')}
      </div>
      {value.mode === 'existing' ? (
        <RoundPicker
          value={value.roundId}
          valueLabel={value.roundLabel}
          onChange={(roundId, round) => set({ roundId, roundLabel: round?.name })}
          disabled={disabled}
        />
      ) : (
        <div className='grid gap-3 sm:grid-cols-2'>
          <div className='space-y-2 sm:col-span-2'>
            <Label htmlFor='round-name'>Tên đợt cấp*</Label>
            <Input
              id='round-name'
              placeholder='Ví dụ: Tốt nghiệp đợt 1 – CNTT K2021 – 07/2026'
              value={value.name}
              maxLength={150}
              disabled={disabled}
              onChange={(e) => set({ name: e.target.value })}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='round-decision'>Số quyết định</Label>
            <Input
              id='round-decision'
              placeholder='Ví dụ: 123/QĐ-HVKTMM'
              value={value.decisionNumber}
              disabled={disabled}
              onChange={(e) => set({ decisionNumber: e.target.value })}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='round-date'>Ngày quyết định</Label>
            <Input
              id='round-date'
              type='date'
              value={value.decisionDate}
              disabled={disabled}
              onChange={(e) => set({ decisionDate: e.target.value })}
            />
          </div>
        </div>
      )}
    </div>
  )
}

export default RoundField
