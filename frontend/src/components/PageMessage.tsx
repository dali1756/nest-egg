import type { ReactNode } from 'react'

function PageMessage({ children }: { children: ReactNode }) {
  return <p className="mx-auto mt-12 max-w-170 px-5 text-muted-foreground">{children}</p>
}

export default PageMessage
