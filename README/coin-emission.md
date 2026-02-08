# Coin Emission

## Currency Model: Integer Microcoins

**Canonical Currency Unit: Microcoins (integer only)**

The economy uses integer microcoins as the sole authoritative currency:

- **1 Coin = 1000 microcoins**
- All storage, math, comparisons, and logs use integer microcoins only
- No floating-point coin values exist at runtime
- Coins are a **display format only** derived from microcoins: `microcoins / 1000` with exactly 3 decimal places
- The UI always displays coins with exactly 3 decimal places, never rounding up

This ensures:
- Perfect precision and auditability
- No rounding errors in economy logic
- Atomic integer operations for all purchases and transfers
- Clean separation between canonical (integer) and display (decimal) representations

---

## Emission System

Coins enter the game only through a server-controlled emission system.

Each season has a global coin emission pool that refills continuously over time.

For each season day:

A maximum global coin budget is defined.

This budget is released gradually across 24 hours.

Coin emission occurs on fixed server ticks.

At each tick:

A small portion of the daily coin budget becomes available.

Available coins are added to the emission pool.

Players earn coins by drawing from this pool via allowed faucets.

Coin faucets include:

Daily login rewards

Capped active play rewards

Daily tasks (post‑alpha)

Limited comeback rewards (post‑alpha)

Each player has a per-day earning cap.

The cap decreases as the season progresses.

Late-season caps are significantly lower.

If the emission pool is low:

Coin earning rates are automatically throttled.

Faucets slow down rather than shutting off abruptly.

Trade burn is a modeled sink (post‑alpha when trading is enabled). The daily emission budget accounts for expected trade burn so liquidity remains sufficient for meaningful daily action. Coin shortage is possible but rare; the economy must not stall.
Burned Coins are removed permanently and never redistributed.

Coins are never created directly by player actions.
Coins cannot exceed the remaining global budget for the day.

All coin grants are validated server-side and recorded in an append-only log.

Star pricing is balanced against emission and average per-player coin availability to keep stars purchasable throughout the season.