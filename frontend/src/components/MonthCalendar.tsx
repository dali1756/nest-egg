import { useState } from 'react'
import { DayButton, DayPicker, type DayButtonProps, type Modifiers } from 'react-day-picker'
import { zhTW } from 'react-day-picker/locale'
import { dayLabel, fromCalendarDay, toCalendarDay } from '../format/date'

const sunday = 0

function dayClassName(isToday: boolean, isSelected: boolean): string {
  const base =
    'mx-auto flex h-11 w-full max-w-12 cursor-pointer flex-col items-center justify-center gap-0.5 rounded-control text-md tabular-nums transition-colors focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default disabled:text-neutral'
  if (isSelected) return `${base} bg-primary font-semibold text-primary-foreground`
  if (isToday) return `${base} border border-brand font-semibold text-primary enabled:hover:bg-brand-soft`
  return `${base} text-heading enabled:hover:bg-brand-soft`
}

function dotClassName(hasEntries: boolean, isSelected: boolean): string {
  if (!hasEntries) return 'bg-transparent'
  return isSelected ? 'bg-primary-foreground' : 'bg-brand'
}

function CalendarDayButton({ modifiers, children, ...props }: DayButtonProps) {
  return (
    <DayButton
      {...props}
      modifiers={modifiers}
      aria-current={modifiers.today ? 'date' : undefined}
      className={dayClassName(modifiers.today, modifiers.selected)}
    >
      {children}
      <span className={`size-1.5 rounded-full ${dotClassName(modifiers.hasEntries, modifiers.selected)}`} />
    </DayButton>
  )
}

function dayButtonLabel(day: Date, modifiers: Modifiers): string {
  const label = dayLabel(fromCalendarDay(day))
  return modifiers.hasEntries ? `${label}，有紀錄` : label
}

const labels = { labelGrid: () => '月曆', labelDayButton: dayButtonLabel }

const components = { DayButton: CalendarDayButton }

const classNames = {
  root: 'mt-4 rounded-card border border-border bg-card px-3 py-3 shadow-card',
  month_caption: 'sr-only',
  month_grid: 'w-full table-fixed',
  weekday: 'pb-1 text-xs font-semibold text-muted-foreground',
  day: 'p-0.5 text-center',
}

function useShownMonth(month: string, onMonthChange: (month: string) => void) {
  const [requestedMonth, setRequestedMonth] = useState(month)
  const [shownMonth, setShownMonth] = useState(month)
  if (requestedMonth !== month) {
    setRequestedMonth(month)
    setShownMonth(month)
  }
  const showMonth = (next: string) => {
    setShownMonth(next)
    onMonthChange(next)
  }
  return [shownMonth, showMonth] as const
}

type MonthCalendarProps = {
  month: string
  today: string
  daysWithEntries: Set<string>
  selectedDate?: string
  onSelect: (date: string) => void
  onMonthChange: (month: string) => void
}

function MonthCalendar({ month, today, daysWithEntries, selectedDate, onSelect, onMonthChange }: MonthCalendarProps) {
  const [shownMonth, showMonth] = useShownMonth(month, onMonthChange)
  const todayDay = toCalendarDay(today)
  return (
    <DayPicker
      mode="single"
      locale={zhTW}
      weekStartsOn={sunday}
      month={toCalendarDay(`${shownMonth}-01`)}
      endMonth={todayDay}
      today={todayDay}
      disabled={{ after: todayDay }}
      selected={selectedDate ? toCalendarDay(selectedDate) : undefined}
      onSelect={(_, clickedDay) => onSelect(fromCalendarDay(clickedDay))}
      onMonthChange={(firstDay) => showMonth(fromCalendarDay(firstDay).slice(0, 7))}
      modifiers={{ hasEntries: [...daysWithEntries].map(toCalendarDay) }}
      labels={labels}
      components={components}
      classNames={classNames}
      hideNavigation
    />
  )
}

export default MonthCalendar
