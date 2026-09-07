import { ButtonLink, EmptyState } from '../components'

interface NotBuiltYetProps {
  /** What this page will be. */
  page: string
  /** The issue that builds it, where one exists. */
  issue?: number
  /** Why there is no issue, where there is not one. */
  note?: string
}

/**
 * NotBuiltYet is the placeholder for a route the scaffold registers but does not
 * yet fill. It uses the real EmptyState rather than a bespoke message, so the
 * scaffold has no second way of saying "nothing here".
 *
 * The two cases are worth keeping apart. A page whose API already exists is
 * waiting on a component; a page with nothing behind it at all is waiting on a
 * phase. Telling a reader the first when the truth is the second is the same
 * mistake as rendering an absent statistic as a zero.
 */
export function NotBuiltYet({ page, issue, note }: NotBuiltYetProps) {
  return (
    <EmptyState
      heading={`The ${page} is not built yet`}
      reason={
        <>
          {issue
            ? `The API behind it exists; the page arrives with issue #${issue}. `
            : `${note} `}
          Until then the coverage table on the home page is the fastest way to see what the
          database holds.
        </>
      }
      action={<ButtonLink to="/">Back to coverage</ButtonLink>}
    />
  )
}
