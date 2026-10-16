import type { ReactNode } from 'react'

type AuthCardProps = {
  title: string
  lead: string
  children: ReactNode
}

function AuthCard({ title, lead, children }: AuthCardProps) {
  return (
    <main className="mx-auto flex max-w-170 justify-center px-5 py-14">
      <section className="w-full max-w-100 rounded-card border border-border bg-card px-7 py-8 shadow-card">
        <h1 className="text-2xl font-bold tracking-[-0.02em] text-heading">{title}</h1>
        <p className="mt-1.5 mb-6 text-sm text-muted-foreground">{lead}</p>
        {children}
      </section>
    </main>
  )
}

export default AuthCard
