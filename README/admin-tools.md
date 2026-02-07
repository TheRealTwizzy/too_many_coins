The system must include a minimal internal admin and observability interface.

Admin access is restricted to authorized accounts only.

---

## Admin Role Philosophy — Sentinels, Not Gods

**Admins are sentinels, not gods.**

The economy must **self-regulate**. Admins provide oversight and emergency safeguards, not active management.

### What Admins MAY Do

Admins may:

- **Emergency pause 1 or all seasons** (temporary freeze)
- **Ban extreme abuse cases** (only after anti-cheat recommendation)
- **Monitor telemetry and economy health** (read-only observability)
- **Advance seasons manually** (recovery only, not normal flow)

### What Admins MUST NOT Do

Admins must NOT:

- Micromanage the economy
- Manually adjust player balances
- Override anti-cheat without justification
- Edit past season data
- Interfere with normal economic flow

**The economy is designed to self-regulate.** Admin intervention must be rare, deliberate, and auditable.

---

## Telemetry Rules — Player-Facing Subset Only

Only telemetry that influences **player decisions** may be shown to players.

### Allowed Player-Facing Telemetry

- **Total Coins in Circulation**: Sum of all player wallet balances (NOT the unreleased Coin Pool)
- **Market pressure trends**: Graphs showing pressure changes over time
- **Star price graphs**: Historical and current pricing
- **Time remaining**: Season countdown and milestones
- **Leaderboard changes**: Rank movements and competitive dynamics

Telemetry may include:

- Graphs
- Timers
- Trend indicators
- Aggregated statistics

### Forbidden Player-Facing Telemetry

Telemetry must NOT expose:

- **Exact internal formulas** (pricing algorithms, emission calculations)
- **Anti-cheat thresholds** (thresholds, multipliers, detection signals)
- **Exploitable signals** (precise emission pool size, exact faucet pacing)
- **Admin control granularity** (exact admin actions beyond high-level pauses)

### Admin-Only Telemetry

Admins have access to:

- Full economy state (emission pool, exact rates, throttle status)
- Anti-cheat signals and abuse event logs
- Player-level detail (balances, earning history, throttle state)
- Trade logs and market activity (when trading is enabled)

Admin telemetry is **never exposed to players**.

---

## Required admin capabilities are split by phase. Alpha is read‑only.

Alpha (read‑only, current build):

Implemented (Alpha):

Season monitoring (single season):

- Active season status (active vs ended).
- Season time remaining (via season snapshot).

Alpha override (admin‑only, recovery):

- Manual season advance when no active season exists or the current season has ended (POST /admin/seasons/advance).
- No parameters are accepted; this is an override, not the normal flow.

Alpha rule:

- “Ending” is internal only; admin UI shows only **Active** or **Ended**.
- When ended, admin economy indicators are read-only and present frozen/final markers (no live emission/inflation rates).
- Admin control strips (pause/freeze/emission controls) are hidden in Alpha.

Economy monitoring (per season):

- Current base star price.
- Current effective star price.
- Current market pressure.
- Daily emission target.
- Daily cap early/late.

Telemetry (current build):

- Event counts per hour by type (from player telemetry stream).
- Notification emit events are logged for observability.
- Emitted event types include: emission_tick, market_pressure_tick, faucet_claim, star_purchase_attempt, star_purchase_success, notification_emitted.
- Admin UI remains read‑only and currently exposes counts, not full raw payloads.

Player inspection (read‑only):

- Player search by username/account/player ID.
- Trust status and flag count.

Abuse monitoring (read‑only):

- Recent abuse events list.
- Anti‑cheat toggle status (visibility only; not configurable).

Auditability (read‑only):

- Star purchase log.
- Admin audit log.

Bug reports (read‑only):

- View bug report list and details (title, description, category, player_id if present, season_id, timestamp, client version if available).
- No editing, deletion, or player responses.
- No attachments in Alpha; admin view is observational only.

Not yet in Alpha (post‑alpha or pending implementation):

- Global coin budget remaining for the day.
- Coin emission rate and throttling state details.
- Coins emitted per hour, coins earned per hour, and average star price over time (beyond event counts).
- Per‑player coin earning history view.
- Per‑player coin and star balance detail view (beyond search results).
- Throttle status per player.
- IP clustering detail views beyond aggregate signals.

Post‑Alpha (planned):

Trading visibility:

Current trade premium and burn rate.

Current trade eligibility tightness.

Stars transferred via trades per hour.

Coins burned via trades per hour.

View trade eligibility status and recent trades.

Safety tools (admin‑only, auditable):

Temporarily pause star purchases per season if needed.

Temporarily reduce coin emission rates.

Freeze a season in emergency cases.

Temporarily disable trading per season if needed.

All admin actions are logged and auditable.