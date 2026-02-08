Stars may only be obtained by purchasing them from the system using coins.

Stars are minted only by the system. Brokered trading may transfer existing Stars between players but never creates new Stars and never bypasses scarcity.

## Star Characteristics

**Stars are NOT tradable in any context** (player-to-player trading, brokered trading, or any other mechanism).

**Stars are NOT spendable** (except via Star Sacrifice for TSAs in Beta+, which permanently destroys them).

**Stars are seasonal competitive units** that:

- Determine leaderboard rank directly
- Reset at season end (do not carry over as currency)
- Convert into a **permanent profile statistic** after season end
- Influence long-term profile rank and identity

**Star value scales with season population**. Stars earned in larger, more competitive seasons carry more weight as a permanent statistic.

**Stars are the permanent score of seasonal performance.**

Star purchases follow these rules:

Players may buy stars one at a time or in bulk.
Bulk purchases are allowed but heavily penalized through scaling.

The total coin cost of a star purchase is determined by:

A base star price that increases over the season

A quantity multiplier that scales non-linearly with purchase size

A market pressure factor based on recent purchase activity

A late-season spike applied during the final week

An affordability guardrail that keeps prices aligned to average coins per player and emission so stars remain purchasable through the season

Quantity scaling:

The quantity multiplier must grow faster than linear.

Large bulk purchases rapidly become inefficient.

Extremely large bulk purchases may include additional hard multipliers.

Bulk purchase interfaces must:

Show the full calculated cost before confirmation

Show how quantity affects price

Warn players when purchases are highly inefficient

Require explicit confirmation

Alpha verification:

- Server recomputes the bulk quote at purchase time; price and balance are re‑checked before commit.
- Bulk warnings are derived from the max bulk multiplier (medium/high/severe thresholds).

Star purchases:

Must be atomic

Must re-check price and balance at confirmation time

Must fail safely if conditions change

Star supply is system-managed and cannot be exhausted.
Scarcity is enforced through pricing, not limited stock.

All star purchases are validated server-side and recorded in an append-only log.

## Star Price Persistence

The current star price is persisted in the season economy table and updated each emission tick (every 60 seconds).

This ensures:
- Star price remains **identically consistent** across all players at any given moment
- Star price remains consistent across server restarts
- Price continuity is maintained during unexpected downtime
- The season-authoritative star price is recoverable from database state

### Star Price Computation (Season-Level Authority)

The star price is computed from **season-level inputs only**. All players see the **identical price** at any given moment in the season.

Season-level inputs:
- Time progression within the season
- Total coins in circulation (across all players)
- Stars purchased this season
- Market pressure (aggregate purchase activity)
- Late-season spike (time-based multiplier)
- Affordability guardrail (derived from total coins / expected player base)

**Active player metrics are NOT a direct input** to star price computation. Player activity influences pricing only indirectly through market pressure (purchase activity).

The computation is performed **once per server tick** and stored in the database. The same value is broadcast to all players via SSE and API endpoints.

If the persisted price is NULL on startup (new season or legacy data), it will be populated by the next emission tick.

### Price Displayed vs. Price Paid

The displayed to all players is the season-authoritative price.

Purchase flow:
- Player sees the authoritative star price
- Anti-abuse logic may affect:
  - Purchase allowance (buying limits)
  - Effective price paid (through cooldowns or other mechanisms)
  - But NOT the displayed price
- Star purchase log records both:
  - season_price_snapshot (authoritative)
  - effective_price_paid (actual cost including anti-abuse adjustments)
