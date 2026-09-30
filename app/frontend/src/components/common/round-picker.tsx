'use client'

import { Check, ChevronsUpDown, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { IssuanceRound, searchIssuanceRounds } from '@/lib/api/digital-degree'
import { cn } from '@/lib/utils/common'

export const ROUND_NONE = 'none'

interface Props {
  value: string
  // Name of the selected round when the caller already knows it.
  valueLabel?: string
  // eslint-disable-next-line no-unused-vars
  onChange: (roundId: string, round: IssuanceRound | null) => void
  // Only suggest rounds that hold diplomas of this faculty.
  facultyId?: string
  // Offer "not attached to a round" (for finding records created before rounds existed).
  allowNone?: boolean
  placeholder?: string
  disabled?: boolean
  className?: string
}

// Suggests the newest rounds as soon as it opens and searches by name as the user
// types, so a university with years of rounds never gets one endless list.
const RoundPicker: React.FC<Props> = (props) => {
  const [open, setOpen] = useState(false)
  const [text, setText] = useState('')
  const [query, setQuery] = useState('')
  const [labels, setLabels] = useState<Record<string, string>>({})

  useEffect(() => {
    const timer = setTimeout(() => setQuery(text.trim()), 250)
    return () => clearTimeout(timer)
  }, [text])

  const rounds = useSWR(open ? ['issuance-rounds', query, props.facultyId || ''] : null, () =>
    searchIssuanceRounds(query, props.facultyId || '', 5)
  )

  useEffect(() => {
    if (!rounds.data?.length) return
    setLabels((prev) => {
      const next = { ...prev }
      rounds.data!.forEach((r) => (next[r.id] = r.name))
      return next
    })
  }, [rounds.data])

  const label =
    props.value === ROUND_NONE ? 'Chưa gắn đợt' : props.value ? labels[props.value] || props.valueLabel || 'Đợt đã chọn' : ''

  const select = (id: string, round: IssuanceRound | null) => {
    if (round) setLabels((prev) => ({ ...prev, [round.id]: round.name }))
    props.onChange(id === props.value ? '' : id, id === props.value ? null : round)
    setOpen(false)
  }

  return (
    <Popover
      open={open}
      onOpenChange={(v) => {
        setOpen(v)
        if (!v) setText('')
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type='button'
          variant='outline'
          role='combobox'
          disabled={props.disabled}
          className={cn(
            'w-full justify-between px-3 py-1 font-normal hover:bg-background',
            !props.value && 'text-muted-foreground hover:text-muted-foreground',
            props.className
          )}
        >
          {/* Not a <span>: the shared Button resizes buttons that contain one (icon + label). */}
          <div className='truncate'>{label || props.placeholder || 'Chọn đợt cấp'}</div>
          {rounds.isLoading ? <Loader2 className='animate-spin' /> : <ChevronsUpDown className='opacity-50' />}
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-[--radix-popover-trigger-width] min-w-[280px] p-0' align='start'>
        {/* Filtering happens on the server, not on the few loaded items. */}
        <Command shouldFilter={false}>
          <CommandInput placeholder='Nhập tên đợt để tìm' className='h-9' value={text} onValueChange={setText} />
          <CommandList>
            {!rounds.isLoading && <CommandEmpty>Không tìm thấy đợt cấp</CommandEmpty>}
            <CommandGroup heading={query ? 'Kết quả tìm kiếm' : 'Đợt cấp gần nhất'}>
              {(rounds.data ?? []).map((round) => (
                <CommandItem key={round.id} value={round.id} onSelect={() => select(round.id, round)}>
                  <div className='flex min-w-0 flex-col'>
                    <span className='truncate'>{round.name}</span>
                    <span className='text-xs text-muted-foreground'>
                      {[round.decision_number, new Date(round.created_at).toLocaleDateString('vi-VN')]
                        .filter(Boolean)
                        .join(' · ')}
                    </span>
                  </div>
                  <Check className={cn('ml-auto', props.value === round.id ? 'opacity-100' : 'opacity-0')} />
                </CommandItem>
              ))}
            </CommandGroup>
            {props.allowNone && !query && (
              <CommandGroup heading='Khác'>
                <CommandItem value={ROUND_NONE} onSelect={() => select(ROUND_NONE, null)}>
                  Chưa gắn đợt
                  <Check className={cn('ml-auto', props.value === ROUND_NONE ? 'opacity-100' : 'opacity-0')} />
                </CommandItem>
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

export default RoundPicker
