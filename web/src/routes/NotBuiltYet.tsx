import { ButtonLink, EmptyState } from '../components'

interface NotBuiltYetProps {
  /** What this page will be. */
  page: string
  /** The issue that builds it. */
  issue: number
}

/**
 * NotBuiltYet is the placeholder for a route the scaffold registers but does not
 * yet fill. It uses the real EmptyState rather than a bespoke message, so the
 * scaffold has no second way of saying "nothing here".
 */
export function NotBuiltYet({ page, issue }: NotBuiltYetProps) {
  return (
    <EmptyState
      heading={`The ${page} is not built yet`}
      reason={
        <>
          The API behind it exists; the page arrives with issue #{issue}. Until then the
          coverage table on the home page is the fastest way to see what the database holds.
        </>
      }
      action={<ButtonLink to="/">Back to coverage</ButtonLink>}
    />
  )
}
