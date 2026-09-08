import { useMemo, useState, type ReactNode } from 'react'
import { AbsentCell } from './AbsentCell'
import styles from './StatTable.module.css'

export type SortValue = number | string | null

export interface Column<Row> {
  key: string
  header: string
  /** Numbers go right, text goes left. */
  align?: 'left' | 'right'
  /**
   * The value to sort on. Return null for a value the source never recorded --
   * null sorts after every present value in both directions, and renders as an
   * AbsentCell unless `render` says otherwise.
   */
  value: (row: Row) => SortValue
  /** Optional custom cell. Only reached when `value` is not null. */
  render?: (row: Row) => ReactNode
  sortable?: boolean
}

interface StatTableProps<Row> {
  /** Always required: a table with no caption is a table nobody can cite. */
  caption: string
  columns: ReadonlyArray<Column<Row>>
  rows: ReadonlyArray<Row>
  rowKey: (row: Row) => string
  defaultSort?: { key: string; direction: Direction }
  /** A totals row, drawn under the heavier rule. */
  aggregate?: ReadonlyArray<ReactNode>
}

type Direction = 'asc' | 'desc'

/**
 * StatTable is hairline rows and tabular numerals, with sortable headers.
 *
 * The one rule that is not cosmetic: an absent value sorts last whichever way
 * the column is sorted. Treating it as zero would collect every unrecorded
 * match at the bottom of an ascending sort and read as the worst performances
 * in the table.
 */
export function StatTable<Row>({
  caption,
  columns,
  rows,
  rowKey,
  defaultSort,
  aggregate,
}: StatTableProps<Row>) {
  const [sort, setSort] = useState<{ key: string; direction: Direction } | null>(
    defaultSort ?? null,
  )

  const sorted = useMemo(() => {
    if (!sort) return rows
    const column = columns.find((c) => c.key === sort.key)
    if (!column) return rows

    const factor = sort.direction === 'asc' ? 1 : -1
    return [...rows].sort((a, b) => {
      const left = column.value(a)
      const right = column.value(b)
      // Absent last, both directions, so the factor is deliberately not applied.
      if (left === null && right === null) return 0
      if (left === null) return 1
      if (right === null) return -1
      if (typeof left === 'string' || typeof right === 'string') {
        return factor * String(left).localeCompare(String(right))
      }
      return factor * (left - right)
    })
  }, [rows, columns, sort])

  function toggle(key: string) {
    setSort((current) =>
      current?.key === key
        ? { key, direction: current.direction === 'asc' ? 'desc' : 'asc' }
        : { key, direction: 'desc' },
    )
  }

  return (
    <div className={styles.wrap}>
      <table className={styles.table}>
        <caption className={styles.caption}>{caption}</caption>
        <thead>
          <tr>
            {columns.map((column) => {
              const active = sort?.key === column.key
              const className = column.align === 'right' ? `${styles.th} ${styles.right}` : styles.th
              return (
                <th
                  key={column.key}
                  scope="col"
                  className={className}
                  aria-sort={active ? (sort.direction === 'asc' ? 'ascending' : 'descending') : 'none'}
                >
                  {column.sortable === false ? (
                    column.header
                  ) : (
                    <button type="button" className={styles.sort} onClick={() => toggle(column.key)}>
                      {column.header}
                      {active ? (
                        <span className={styles.marker} aria-hidden="true">
                          {sort.direction === 'asc' ? ' ↑' : ' ↓'}
                        </span>
                      ) : null}
                    </button>
                  )}
                </th>
              )
            })}
          </tr>
        </thead>
        <tbody>
          {sorted.map((row) => (
            <tr key={rowKey(row)} className={styles.row}>
              {columns.map((column) => {
                const value = column.value(row)
                const className =
                  column.align === 'right' ? `${styles.td} ${styles.right}` : styles.td
                return (
                  <td key={column.key} className={className}>
                    {value === null ? (
                      <AbsentCell label={column.header} />
                    ) : column.render ? (
                      column.render(row)
                    ) : (
                      value
                    )}
                  </td>
                )
              })}
            </tr>
          ))}
          {aggregate ? (
            <tr className={styles.aggregate}>
              {aggregate.map((cell, index) => {
                const column = columns[index]
                const className =
                  column?.align === 'right' ? `${styles.td} ${styles.right}` : styles.td
                return (
                  <td key={column?.key ?? index} className={className}>
                    {cell}
                  </td>
                )
              })}
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  )
}
