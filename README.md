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

| File                           | Source                                        |
| ------------------------------ | --------------------------------------------- |
| `data/sorcery/cards.json`      | The public sorcerytcg.com card catalog        |
| `data/sorcery/proxies.yaml`    | The wants and the PDF flags, edited by hand   |
| `data/sorcery/collection.json` | The owned cards, from the sorcerytcg.com site |
| `data/sorcery/owned.yaml`      | The owned cards, edited by hand               |
| `data/sorcery/printed.json`    | The ledger of printed runs                    |

A run uses `owned.yaml` only if `collection.json` does not exist.

## Setup

1. Install Go and mage.
2. Clone silhouette-card-maker to
   `~/Code/github.com/Alan-Cha/silhouette-card-maker`, and make its `venv` as
   its README says. Set `SILHOUETTE_CARD_MAKER` to use a different path.
3. Optional: scan a spellbook card back and an atlas card back. Save them as
   `backs/sorcery/spellbook.png` and `backs/sorcery/atlas.png`. Without them,
   the PDF prints fronts only. Git ignores these files.

## Update the data

Download the card catalog:

```bash
mage sorcery:catalog
```

To read the owned cards from the sorcerytcg.com collection, add the session
cookie to the macOS Keychain once:

1. Sign in to sorcerytcg.com in a browser.
2. Open the developer tools, and copy the `Cookie` request header of a
   sorcerytcg.com request.
3. Run this command, and paste the cookie at the prompt:

   ```bash
   security add-generic-password -a "$USER" -s card-proxies-sorcerytcg -w
   ```

Then download the collection:

```bash
mage sorcery:collection
```

The collection uses an endpoint that the sorcerytcg.com pages use. It is not a
documented API, so it can change without notice. If the target fails, use
`owned.yaml` instead.

Commit the JSON files that the targets write.

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

- The Alpha, Beta and Dragonlord card images are 380 pixels wide, about 150
  PPI at card size. They print soft. The stage report lists each low
  resolution image.
- A card image shows the printed rules text. A proxy of an errata card shows
  the new text only if a newer printing has it.
- Double-faced cards, such as Druid, print the front face only.
