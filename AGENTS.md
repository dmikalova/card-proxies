# AGENTS.md

Conventions for working on card-proxies. Use the vocabulary in `CONTEXT.md`
(catalog, printing, want group, collection, plan, run, ledger) in code and
comments.

## Validating

Run `mage ci:fix && mage ci:check` before calling work done. It must print
`ALL GREEN`. After a change to `proxies.yaml` or the plan code, also run
`mage sorcery:plan` and read the totals.

## Rules

- **The repository is public.** Commit card data as JSON. Keep card images,
  card back scans and session cookies out of git: images go in `cache/` and
  `runs/`, backs in `backs/`, and the cookie in the Keychain.
- **Mage targets write the JSON in `data/`.** Change `cards.json`,
  `collection.json` and `printed.json` through their targets, then commit the
  result. Edit `proxies.yaml` and `owned.yaml` by hand.
- **`cards.json` holds only the fields of `Card`.** The official rules text
  has typos, and `ci:spell` rewrites them in a committed copy. Add a field to
  `Card` to keep it.
- **The card image CDN serves Sites in portrait.** Card fronts are copied as
  they are; only the atlas back turns to portrait.
- **`inventory.getQuantities` is not a documented API.** It answers by
  printing ID for the card IDs it gets. Keep its parsing strict, so that a
  change on sorcerytcg.com fails loudly.
