Stars may only be obtained by purchasing them from the system using coins.

Star purchases follow these rules:

Players may buy stars one at a time or in bulk.
Bulk purchases are allowed but heavily penalized through scaling.

The total coin cost of a star purchase is determined by:

A base star price that increases over the season

A quantity multiplier that scales non-linearly with purchase size

A market pressure factor based on recent purchase activity

A late-season spike applied during the final week

Quantity scaling:

The quantity multiplier must grow faster than linear.

Large bulk purchases rapidly become inefficient.

Extremely large bulk purchases may include additional hard multipliers.

Bulk purchase interfaces must:

Show the full calculated cost before confirmation

Show how quantity affects price

Warn players when purchases are highly inefficient

Require explicit confirmation

Star purchases:

Must be atomic

Must re-check price and balance at confirmation time

Must fail safely if conditions change

Star supply is system-managed and cannot be exhausted.
Scarcity is enforced through pricing, not limited stock.

All star purchases are validated server-side and recorded in an append-only log.