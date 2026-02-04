Players earn coins through a limited set of server-controlled faucets.

Daily Login:

Available once per 20 hours per season.

Grants a small, fixed amount of coins.

Designed to reward consistency, not grinding.

Daily Tasks:

Post‑alpha only.

A small set of simple tasks refreshed at daily reset.

Tasks grant moderate coin rewards.

Completing all tasks does not exceed the player daily earning cap.

Active Play:

Coins are granted at a slow, steady rate during active participation.

Active play rewards are capped per hour and per day.

AFK or idle behavior does not generate coins.

Alpha runtime default: passive drip is disabled (`drip_enabled=false`).

Comeback Reward:

Post‑alpha only.

Available only to players who have been inactive for a defined period.

Grants a one-time, modest coin boost.

Cannot exceed a fixed percentage of the daily earning cap.

Caps:

Each player has a daily coin earning cap per season.

The cap decreases as the season progresses.

Late-season caps are significantly lower than early-season caps.

Even late-season caps allow some daily earning; faucets never drop to zero.

All faucet grants draw from the global emission pool.
If the pool is low, faucet rewards are proportionally throttled.

Faucet tuning is balanced against trade burn to keep the economy liquid enough for daily action.
Coin shortage is possible but rare.

All faucet usage and coin grants are validated server-side and logged.

Login Playability Safeguard (Alpha):

If a player logs in with a balance too low to make near-term progress,
the server may top them up to a minimum playable balance.

This safeguard:

Draws from the global emission pool
Has a short cooldown
Is intended to keep the game playable within minutes
May bypass the per-day earning cap as an alpha-only safety net