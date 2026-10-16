import type { ReactNode } from 'react'

const tones = {
  error: 'bg-destructive-soft text-destructive-text',
  success: 'bg-success-soft text-success-text',
}

function Notice({ tone, children }: { tone: keyof typeof tones; children: ReactNode }) {
  return (
    <p
      role={tone === 'error' ? 'alert' : 'status'}
      className={`rounded-control px-3.5 py-2.5 text-sm font-semibold ${tones[tone]}`}
    >
      {children}
    </p>
  )
}

export default Notice
