import type { ReactNode } from 'react'

type AuthCardProps = {
  title: string
  lead: string
  children: ReactNode
}

function AuthCard({ title, lead, children }: AuthCardProps) {
  return (
    <main className="container auth-page">
      <section className="card auth-card">
        <h1 className="auth-title">{title}</h1>
        <p className="auth-lead">{lead}</p>
        {children}
      </section>
    </main>
  )
}

export default AuthCard
