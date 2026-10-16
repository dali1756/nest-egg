import type { ComponentProps } from 'react'
import { Link } from 'react-router'

function TextLink({ className = '', ...props }: ComponentProps<typeof Link>) {
  return (
    <Link
      className={`ml-1 rounded-sm font-semibold text-primary hover:underline focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring ${className}`}
      {...props}
    />
  )
}

export default TextLink
