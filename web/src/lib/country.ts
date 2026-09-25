/**
 * The database stores IOC country codes, because the tour's own files do. They
 * are not ISO 3166-1 codes and the differences are not cosmetic: Germany is GER
 * and not DE, Switzerland SUI and not CH, the Netherlands NED and not NL. The
 * flag files are named by ISO code, so this is the one place that knows how to
 * get from one to the other.
 *
 * A code with no entry here gets no flag and keeps its letters, which is the
 * right answer twice over. Codes this list has never seen are new or mistyped,
 * and flying a guess at them would be worse than flying nothing; and the
 * historical codes below have no modern flag to fly at all.
 */
const ISO: Record<string, string> = {
  AFG: 'af', ALB: 'al', ALG: 'dz', AND: 'ad', ANG: 'ao', ANT: 'ag', ARG: 'ar',
  ARM: 'am', ARU: 'aw', ASA: 'as', AUS: 'au', AUT: 'at', AZE: 'az',
  BAH: 'bs', BAN: 'bd', BAR: 'bb', BDI: 'bi', BEL: 'be', BEN: 'bj', BER: 'bm',
  BHU: 'bt', BIH: 'ba', BIZ: 'bz', BLR: 'by', BOL: 'bo', BOT: 'bw', BRA: 'br',
  BRN: 'bh', BRU: 'bn', BUL: 'bg', BUR: 'bf',
  CAF: 'cf', CAM: 'kh', CAN: 'ca', CAY: 'ky', CGO: 'cg', CHA: 'td', CHI: 'cl',
  CHN: 'cn', CIV: 'ci', CMR: 'cm', COD: 'cd', COK: 'ck', COL: 'co', COM: 'km',
  CPV: 'cv', CRC: 'cr', CRO: 'hr', CUB: 'cu', CYP: 'cy', CZE: 'cz',
  DEN: 'dk', DJI: 'dj', DMA: 'dm', DOM: 'do',
  ECU: 'ec', EGY: 'eg', ERI: 'er', ESA: 'sv', ESP: 'es', EST: 'ee', ETH: 'et',
  FIJ: 'fj', FIN: 'fi', FRA: 'fr', FSM: 'fm',
  GAB: 'ga', GAM: 'gm', GBR: 'gb', GBS: 'gw', GEO: 'ge', GEQ: 'gq', GER: 'de',
  GHA: 'gh', GRE: 'gr', GRN: 'gd', GUA: 'gt', GUI: 'gn', GUM: 'gu', GUY: 'gy',
  HAI: 'ht', HKG: 'hk', HON: 'hn', HUN: 'hu',
  INA: 'id', IND: 'in', IRI: 'ir', IRL: 'ie', IRQ: 'iq', ISL: 'is', ISR: 'il',
  ISV: 'vi', ITA: 'it', IVB: 'vg',
  JAM: 'jm', JOR: 'jo', JPN: 'jp',
  KAZ: 'kz', KEN: 'ke', KGZ: 'kg', KIR: 'ki', KOR: 'kr', KOS: 'xk', KSA: 'sa',
  KUW: 'kw',
  LAO: 'la', LAT: 'lv', LBA: 'ly', LBN: 'lb', LBR: 'lr', LCA: 'lc', LES: 'ls',
  LIE: 'li', LTU: 'lt', LUX: 'lu',
  MAD: 'mg', MAR: 'ma', MAS: 'my', MAW: 'mw', MDA: 'md', MDV: 'mv', MEX: 'mx',
  MGL: 'mn', MKD: 'mk', MLI: 'ml', MLT: 'mt', MNE: 'me', MON: 'mc', MOZ: 'mz',
  MRI: 'mu', MTN: 'mr', MYA: 'mm',
  NAM: 'na', NCA: 'ni', NED: 'nl', NEP: 'np', NGR: 'ng', NIG: 'ne', NOR: 'no',
  NZL: 'nz',
  OMA: 'om',
  PAK: 'pk', PAN: 'pa', PAR: 'py', PER: 'pe', PHI: 'ph', PLE: 'ps', PLW: 'pw',
  PNG: 'pg', POL: 'pl', POR: 'pt', PRK: 'kp', PUR: 'pr',
  QAT: 'qa',
  ROU: 'ro', RSA: 'za', RUS: 'ru', RWA: 'rw',
  SAM: 'ws', SEN: 'sn', SEY: 'sc', SIN: 'sg', SKN: 'kn', SLE: 'sl', SLO: 'si',
  SMR: 'sm', SOL: 'sb', SOM: 'so', SRB: 'rs', SRI: 'lk', SSD: 'ss', STP: 'st',
  SUD: 'sd', SUI: 'ch', SUR: 'sr', SVK: 'sk', SWE: 'se', SWZ: 'sz', SYR: 'sy',
  TAN: 'tz', TGA: 'to', THA: 'th', TJK: 'tj', TKM: 'tm', TLS: 'tl', TOG: 'tg',
  TPE: 'tw', TTO: 'tt', TUN: 'tn', TUR: 'tr', TUV: 'tv',
  UAE: 'ae', UGA: 'ug', UKR: 'ua', URU: 'uy', USA: 'us', UZB: 'uz',
  VAN: 'vu', VEN: 've', VIE: 'vn', VIN: 'vc',
  YEM: 'ye',
  ZAM: 'zm', ZIM: 'zw',
}

/**
 * Countries that stopped existing. The files are full of them -- the database
 * has URS, YUG and TCH players who never played under any other flag -- and
 * none has a current ISO code. Listed rather than left out so the difference
 * between "no flag exists" and "this code is unknown to us" stays visible in
 * the source, even though both render the same way.
 *
 * Successor states are deliberately not substituted. A Soviet player did not
 * play for Russia, and a Yugoslav one did not play for Serbia.
 */
const HISTORICAL = new Set([
  'URS', // Soviet Union
  'TCH', // Czechoslovakia
  'YUG', // Yugoslavia
  'SCG', // Serbia and Montenegro
  'FRG', // West Germany
  'GDR', // East Germany
  'AHO', // Netherlands Antilles
  'RHO', // Rhodesia
  'ZAI', // Zaire
  'BIR', // Burma
  'CEY', // Ceylon
  'EUA', // United Team of Germany
  'UNK', // the sources' own marker for no country at all
])

/** The ISO alpha-2 code a flag file is named by, or null when there is none. */
export function flagCode(ioc: string | null | undefined): string | null {
  if (ioc === null || ioc === undefined) return null
  const code = ioc.trim().toUpperCase()
  if (code === '' || HISTORICAL.has(code)) return null
  return ISO[code] ?? null
}

/** True for a code this list knows about but has no flag for. */
export function isHistorical(ioc: string | null | undefined): boolean {
  if (ioc === null || ioc === undefined) return false
  return HISTORICAL.has(ioc.trim().toUpperCase())
}
