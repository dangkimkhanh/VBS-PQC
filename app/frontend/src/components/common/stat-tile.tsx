import React from 'react'
import { cn } from '@/lib/utils/common'

interface StatTileProps {
  icon: React.ReactNode
  label: string
  value: React.ReactNode
  children?: React.ReactNode
  className?: string
}

// A single headline number with its label and an optional short breakdown.
export const StatTile: React.FC<StatTileProps> = ({ icon, label, value, children, className }) => (
  <section className={cn('rounded-xl border bg-card p-5 shadow-sm', className)}>
    <div className='flex items-start justify-between gap-3'>
      <p className='text-sm font-medium text-muted-foreground'>{label}</p>
      <span className='rounded-lg bg-blue-50 p-2 text-main dark:bg-blue-950/60 [&_svg]:size-4' aria-hidden>
        {icon}
      </span>
    </div>
    <p className='mt-1 text-3xl font-semibold tabular-nums tracking-tight'>{value}</p>
    {children && <div className='mt-2 space-y-0.5 text-sm text-muted-foreground [&_p]:text-sm'>{children}</div>}
  </section>
)

interface ProgressRowProps {
  label: string
  value: number
  total: number
}

// Share of a total, written out in text so the bar is never the only carrier.
export const ProgressRow: React.FC<ProgressRowProps> = ({ label, value, total }) => {
  const percent = total > 0 ? Math.round((100 * value) / total) : 0
  return (
    <div>
      <div className='mb-1.5 flex items-baseline justify-between gap-2 text-sm'>
        <span>{label}</span>
        <span className='tabular-nums text-muted-foreground'>
          {value.toLocaleString('vi-VN')}/{total.toLocaleString('vi-VN')} · {percent}%
        </span>
      </div>
      <div
        className='h-2 overflow-hidden rounded-full bg-muted'
        role='progressbar'
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
      >
        <div className='h-full rounded-full bg-main transition-all' style={{ width: `${percent}%` }} />
      </div>
    </div>
  )
}

export const panelClass = 'rounded-xl border bg-card p-5 shadow-sm'
