# card-proxies

Proxy print runs for the trading cards that I do not own, or that need new
rules text. Each run prints only the cards that are still missing. The tasks
are [mage](https://magefile.org) targets, and
[silhouette-card-maker](https://github.com/Alan-Cha/silhouette-card-maker)
makes the PDF. The vocabulary is in [`CONTEXT.md`](CONTEXT.md).

The first game is Sorcery: Contested Realm.

## How a run works

A run prints, for each card: the want, minus the owned copies, minus the
copies that earlier runs printed.

| File                          | Source                                         |
| ----------------------------- | ---------------------------------------------- |
| `data/sorcery/cards.json`     | The public sorcerytcg.com card catalog         |
| `data/sorcery/tcgplayer.json` | Alpha and Beta printings to TCGplayer products |
| `data/sorcery/proxies.toml`   | The want groups and the PDF flags, by hand     |
| `data/sorcery/owned.toml`     | The owned groups, by hand                      |
| `data/sorcery/printed.json`   | The ledger of printed runs, from the target    |

A group selects cards by name, set, product and rarity. Each card gets a
playset by rarity unless the group sets a quantity: Ordinary 4, Exceptional
3, Elite 2, and 1 for Uniques, Avatars and tokens.

## Setup

1. Install Go and mage.
2. Clone silhouette-card-maker to
   `~/Code/github.com/Alan-Cha/silhouette-card-maker`, and make its `venv` as
   its README says. Set `SILHOUETTE_CARD_MAKER` to use a different path.
3. Optional: save a spellbook card back and an atlas card back as
   `backs/sorcery/spellbook` and `backs/sorcery/atlas`, each a `.png`, `.jpg`
   or `.jpeg`. Without them, the PDF prints fronts only. Git ignores these
   files.

## Update the catalog

```bash
mage sorcery:catalog
```

Commit `data/sorcery/cards.json`. A diff shows the new cards and printings.

## Print a run

1. Run `mage sorcery:plan` to see the cards of the next run. It changes
   nothing.
2. Run `mage sorcery:stage`. It downloads the card images and writes the run
   to `runs/sorcery/<date-time>/`.
3. Run `mage sorcery:pdf`. It makes `sorcery.pdf` in the run directory.
4. Print and cut the cards.
5. Run `mage sorcery:record`, and commit `data/sorcery/printed.json`. The next
   run skips these cards.

`mage sorcery:run` does steps 2 and 3.

## Known limits

- The official Alpha, Beta and Dragonlord card images are 380 pixels wide,
  about 150 PPI at card size. For Alpha and Beta, a run uses the 750 x 1048
  TCGplayer image instead, with the corners of an official card image. Run
  `mage sorcery:tcgplayer` to update `data/sorcery/tcgplayer.json`, the map of
  printings to TCGplayer products. Dragonlord, Crusade and Jihad have no
  sharper image and print soft. The stage report lists each low resolution
  image.
- A card image shows the printed rules text. A proxy of an errata card shows
  the new text only if a newer printing has it.
- Double-faced cards, such as Druid, print the front face only.
