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

Refreshing daily tasks

Daily reset logic must be guarded so it runs exactly once per season-day.

Season Time:

Each season has a fixed start_time and end_time.

Season day index is derived from (current_time - start_time).

Pricing curves, coin budgets, and caps are based on the derived season day.

Season end transitions the season to ended state and freezes economy actions.

Clients receive all time-related values from the server and never calculate time-sensitive logic locally.