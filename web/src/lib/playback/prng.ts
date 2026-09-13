/**
 * mulberry32: a small seeded generator, so a watched match can be replayed
 * exactly and a test can pin what it saw. Returns floats in [0, 1).
 */
export function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

/** A fresh seed for a new match. Six digits, so it reads as a number and not a hash. */
export function newSeed(): number {
  return 1 + Math.floor(Math.random() * 999999)
}
