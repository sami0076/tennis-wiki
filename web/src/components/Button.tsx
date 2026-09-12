import type { ButtonHTMLAttributes } from 'react'
import { Link } from 'react-router-dom'
import styles from './Button.module.css'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement>

/**
 * Button is a typed label boxed in one rule. The label says what happens --
 * "Simulate this matchup", never "Go", and never with an arrow appended.
 */
export function Button({ className, type = 'button', ...rest }: ButtonProps) {
  return <button type={type} className={[styles.button, className].filter(Boolean).join(' ')} {...rest} />
}

interface ButtonLinkProps {
  to: string
  children: React.ReactNode
}

/** ButtonLink is the same control when the action is going somewhere. */
export function ButtonLink({ to, children }: ButtonLinkProps) {
  return (
    <Link to={to} className={styles.button}>
      {children}
    </Link>
  )
}
