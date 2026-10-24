import type { ButtonHTMLAttributes } from 'react'
import { buttonClassName, type ButtonStyle } from './buttonStyles'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & ButtonStyle

function Button({ variant, block, ...props }: ButtonProps) {
  return <button className={buttonClassName({ variant, block })} {...props} />
}

export default Button
