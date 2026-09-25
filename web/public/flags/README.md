# Flags

265 SVG flags from [country-flag-icons](https://github.com/catamphetamine/country-flag-icons),
MIT licensed (see `LICENSE`), in 3:2 and named by their **ISO 3166-1 alpha-2** code.

The database stores **IOC** codes, which are not the same thing — GER not DE, SUI not CH,
NED not NL — so `web/src/lib/country.ts` maps one to the other and is the only place that
knows about the difference.

Served as static files rather than bundled: a page shows a handful of flags, and only those
are fetched. Nothing here is imported by the JavaScript.
