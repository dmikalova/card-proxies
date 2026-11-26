# AGENTS.md

Conventions for working on card-proxies. Use the vocabulary in `CONTEXT.md`
(catalog, printing, group, playset, plan, run, ledger) in code and comments.

## Validating

Run `mage ci:fix && mage ci:check` before calling work done. It must print
`ALL GREEN`. After a change to a `.toml` file or the plan code, also run
`mage sorcery:plan` and read the totals.

## Rules

- **The repository is public.** Commit card data as JSON. Keep card images
  and the official card back scans out of git: images go in `cache/` and
  `runs/`, back scans in `backs/`. The proxy backs
  `backs/sorcery/*-claude.png` are the only committed images.
- **Hand-edited files are TOML; generated files are JSON.** Edit
  `proxies.toml` and `owned.toml` by hand. Change `cards.json` and
  `printed.json` only through their mage targets, then commit the result.
- **`cards.json` holds only the fields of `Card`.** The official rules text
  has typos, and `ci:spell` rewrites them in a committed copy. Add a field to
  `Card` to keep it.
- **Card images show the printed rules text.** An errata proxy needs a
  printing whose image has the new text. Check the image before you add a card
  to the errata group.
- **tcgcsv.com refuses Go's default User-Agent.** Send a User-Agent header on
  every request to it.
- **The card image CDN serves Sites in portrait.** Card fronts are copied as
  they are; only the atlas back turns to portrait.
