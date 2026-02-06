The server is the sole authority over time.

The system uses three time layers:

Tick Time:

A fixed server tick runs every 60 seconds.

Tick time is used for coin emission, throttling adjustments, and market pressure smoothing.

Tick execution is idempotent and safe to retry.

Daily Time:

Each season has a daily boundary based on UTC.

Daily resets occur once per day per season.

Daily resets include:

Resetting per-player daily earning totals

Refreshing daily tasks (post‑alpha)

Daily reset logic must be guarded so it runs exactly once per season-day.

Season Time:

Each season has a fixed start_time and end_time, with length defined by server phase:

- Alpha: 14 days by default, extendable up to 21 days only with explicit telemetry-gap configuration.
- Beta: 28 days.
- Release: 28 days.

Season day index is derived from the UTC date difference between the season start day and the current day.

Server-authoritative time fields must be provided to clients, including:

- season start time
- season end time
- day index (1-based)
- total days (phase-bound)
- remaining seconds (0 when ended)

Ended invariants (mandatory):

- `status = ended` implies `remaining_seconds = 0`.
- `day_index = total_days`.
- Live economy fields are omitted; only final snapshot fields are provided.

Clients must render these values directly and must not infer total days or day index locally.

Pricing curves, coin budgets, and caps are based on the derived season day.
Trade eligibility bands, premiums, and burn rates are also based on the derived season day.

Season end transitions the season to ended state and freezes economy actions.

Alpha auto-advance (server-only):

When the active Alpha season ends by time, the server finalizes snapshots and immediately starts a new Alpha season.
Active season ID and start time are persisted so restarts resume the same season.
Manual admin advance is override-only and not the normal flow.

Clients receive all time-related values from the server and never calculate time-sensitive logic locally.