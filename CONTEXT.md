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

**Want group**:
A named group of cards to proxy, with a quantity for each card, in
`data/<game>/proxies.yaml`.
_Avoid_: bucket, list

**Collection**:
The cards that are owned, by printing.
_Avoid_: inventory

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
