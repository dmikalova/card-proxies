# card-proxies

Proxy print runs for trading cards that are missing from a physical
collection.

## Language

**Catalog**:
Every card of a game with every printing of it, from the game's public card
data. The committed copy is `data/<game>/cards.json`.
_Avoid_: database, card list

**Printing**:
One printing of a card: one set, finish and product. A run prints the image of
one printing of each card.
_Avoid_: variant, version

**Group**:
A named selection of cards, by name, set, product and rarity, with a number of
copies of each card. A **want group** in `proxies.toml` asks for cards; an
**owned group** in `owned.toml` records cards that are owned.
_Avoid_: bucket, list

**Playset**:
The copies of a card that the deck limits allow, by rarity: Ordinary 4,
Exceptional 3, Elite 2, and 1 for every other card. A group gets a playset of
each card unless it sets a quantity.

**Plan**:
For each wanted card: the want, the owned and printed copies, and the copies
that the next run prints.
_Avoid_: manifest

**Run**:
One staged print job: the front images, the card backs and the plan, in
`runs/<game>/<date-time>/`.
_Avoid_: batch, job

**Ledger**:
The record of every run that was printed and cut, in `data/<game>/printed.json`.
_Avoid_: history, log

**Spellbook card** and **atlas card**:
The two card backs of Sorcery. Sites are atlas cards; every other card type is
a spellbook card.
