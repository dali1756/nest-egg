import type { ButtonHTMLAttributes } from 'react'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'quiet'
  block?: boolean
}

const variants = {
  primary: 'bg-primary text-primary-foreground enabled:hover:bg-primary-hover',
  quiet: 'flex-none border border-border bg-card text-primary enabled:hover:border-primary',
}

function Button({ variant = 'primary', block = false, ...props }: ButtonProps) {
  const size = block ? 'w-full px-4 py-2.75 text-md' : variant === 'quiet' ? 'px-3.5 py-1.5 text-sm' : 'px-4 py-2 text-sm'
  return (
    <button
      className={`cursor-pointer rounded-control font-semibold transition-colors focus-visible:outline-3 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-default disabled:opacity-60 ${variants[variant]} ${size}`}
      {...props}
    />
  )
}

export default Button
