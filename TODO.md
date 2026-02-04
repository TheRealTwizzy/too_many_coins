# TODO — Canonical Execution Plan (Single Source of Truth)

This document supersedes any prior task list. It is ordered by dependency and reflects current code + README canon as of February 3, 2026.

Legend: each task is explicitly marked with a status tag.

Status Tags:
- [DONE]
- [ALPHA REQUIRED]
- [ALPHA EXECUTION]
- [POST-ALPHA]

---

## Alpha Exit Criteria (Non‑Negotiable)
- [ ] [ALPHA REQUIRED] Players can sign up, log in, and enter the active season without manual admin steps (single season is acceptable for Alpha).
- [ ] [ALPHA REQUIRED] Economy runs continuously without admin intervention (tick loop, emission, star pricing, market pressure updates).
- [ ] [ALPHA REQUIRED] Daily play loop works: daily login faucet + active play faucet + decreasing daily cap + emission pool enforcement.
- [ ] [ALPHA REQUIRED] Login playability safeguard keeps new/returning players playable within minutes (alpha‑only, emission‑pool backed).
- [ ] [ALPHA REQUIRED] Players can observe the economy clearly in UI/SSE (time remaining, current star price, coins in circulation, market pressure, emission cadence).
- [ ] [ALPHA REQUIRED] Star purchase flow works end‑to‑end (single + bulk), shows warnings, and is atomic.
- [ ] [ALPHA REQUIRED] Season end freezes all economy actions and writes end‑of‑season snapshots.
- [ ] [ALPHA REQUIRED] Known missing systems are explicitly labeled in UI/docs (trading, multi‑season, cosmetics, etc.).

Allowed to be rough or missing in Alpha:
- Trading system and trading UI
- Multi‑season runtime and season lobby
- Cosmetics, badges, titles, and long‑term progression
- Profile collections, settings, and accessibility polish
- Admin analytics polish beyond core monitoring

---

## Phase 0 — Canon & Reality Reconciliation (Must Stay Honest)
- [x] [DONE] 0.1 Audit repository vs canon sources (README set + SPEC + alpha‑execution + first‑playable)
- [x] [DONE] 0.2 Resolve passive drip contradiction (code runs drip; canon says post‑alpha)
  - [x] [DONE] Decide Alpha stance: disable drip via global_settings.drip_enabled default false OR update canon to allow drip
  - [x] [DONE] Document operational default for alpha (global_settings + runtime log confirms drip disabled/enabled)
- [x] [DONE] 0.3 Resolve admin‑tools overreach (docs list views not present in admin endpoints/UI)
  - [x] [DONE] Audit Alpha admin UI/endpoints vs docs (coin budget remaining, coin earning history, trust/throttle status, IP clustering detail)
  - [x] [DONE] Mark missing views as post‑alpha or add explicit implementation tasks
- [x] [DONE] 0.4 Resolve anti‑abuse doc vs code (CAPTCHA/verification claimed but not implemented)
  - [x] [DONE] Docs already label CAPTCHA/verification as post‑alpha
- [x] [DONE] 0.5 Resolve persistent‑state doc vs schema (seasons/player‑season tables not present)
  - [x] [DONE] persistent‑state.md already marks schema expansion as post‑alpha unless implemented
- [x] [DONE] 0.6 Document login playability safeguard in canon docs
- [x] [DONE] 0.7 Resolve telemetry event naming mismatch (alpha‑execution says join_season; client emits login)
  - [x] [DONE] Decide canonical event names for Alpha and update telemetry contract + TODO list

---

## Phase 1 — Backend Foundations (Authoritative Core)
- [x] [DONE] 1.1 Go + net/http backend
- [x] [DONE] 1.2 PostgreSQL persistence
- [x] [DONE] 1.3 Base schema tables
  - [x] [DONE] players, accounts, sessions
  - [x] [DONE] season_economy persistence
  - [x] [DONE] star_purchase_log, notifications
- [x] [DONE] 1.4 Verify schema is aligned with canonical entities (coin_earning_log, abuse_events, telemetry)

---

## Phase 2 — Time System & Season Core
- [x] [DONE] 2.1 Fixed phase‑bound season clock
- [x] [DONE] 2.2 60s tick loop for emission + pressure
- [x] [DONE] 2.3 Season end snapshot on tick
- [x] [DONE] 2.4 Validate time semantics (season day index, reset boundaries, end‑state gating)
- [ ] [POST-ALPHA] 2.5 Multi‑season runtime model (seasons table, staggered starts, per‑season tick scheduling)

## Phase Transition Tasks (Explicit)
- [ ] [ALPHA REQUIRED] Alpha → Beta: introduce phase config (`PHASE`) and verify Beta season length (28 days) with 2–3 staggered/overlapping seasons (runtime model remains post‑alpha until 2.5 is implemented).
- [ ] [POST-ALPHA] Beta → Release: remove Alpha-only safeguards (single-season lock, alpha extension gates) after multi‑season runtime is stable and verified.

---

## Phase 3 — Economy Emission & Pools
- [x] [DONE] 3.1 Emission pool and daily budget
- [x] [DONE] 3.2 Emission time‑sliced per tick
- [x] [DONE] 3.3 Emission throttling via pool availability
- [x] [DONE] 3.3a Align emission curve to runtime season length (Alpha 14 days / extension-aware)
- [ ] [ALPHA REQUIRED] 3.4 Validate emission pacing vs coin‑emission.md (daily budget, smooth throttle, no abrupt stops)
- [ ] [ALPHA REQUIRED] 3.5 Validate emission floor (prevents pool starvation while respecting scarcity)

---

## Phase 4 — Faucets & Daily Earnings
- [x] [DONE] 4.1 Daily login faucet (cooldown + log + emission cap)
- [x] [DONE] 4.2 Activity faucet (cooldown + log + emission cap)
- [x] [DONE] 4.3 Per‑player daily earn cap with seasonal decay
- [x] [DONE] 4.4 Append‑only coin earning log (source_type + amount)
- [x] [DONE] 4.5 Login playability safeguard (alpha‑only, emission‑pool backed, short cooldown)
- [x] [DONE] 4.6 Verify login safeguard behavior (min balance target, cooldown, no daily‑cap dead‑locks)
- [x] [DONE] 4.7 Confirm faucet priorities and pool gating match canon (no player‑created coins)
- [x] [DONE] 4.8 Resolve passive drip status (enabled vs disabled for Alpha)
- [ ] [POST-ALPHA] 4.9 Daily tasks faucet
- [ ] [POST-ALPHA] 4.10 Comeback reward faucet

---

## Phase 5 — Pricing & Purchases
- [x] [DONE] 5.1 Server‑authoritative star pricing
  - [x] [DONE] Time pressure + late‑season spike
  - [x] [DONE] Quantity scaling for bulk purchases
  - [x] [DONE] Market pressure multiplier + caps
  - [x] [DONE] Affordability guardrail
- [x] [DONE] 5.2 Atomic star purchases (single + bulk)
- [x] [DONE] 5.2a Align pricing time progression to runtime season length (Alpha 14 days / extension-aware)
- [ ] [ALPHA REQUIRED] 5.3 Validate pricing curves vs coin emission (affordability and late‑season scarcity)
- [ ] [ALPHA REQUIRED] 5.4 Validate bulk purchase warnings and re‑check at confirmation

---

## Phase 6 — Market Pressure
- [x] [DONE] 6.1 Market pressure computed server‑side
- [x] [DONE] 6.2 Rate‑limited adjustments per tick
- [ ] [ALPHA REQUIRED] 6.3 Validate pressure inputs vs canon (no trade inputs until trading exists)
- [ ] [ALPHA REQUIRED] 6.4 Validate pressure appears in SSE + UI and is stable under bursts

---

## Phase 7 — Telemetry, Calibration, and Economic History (Data‑Driven Only)
- [x] [DONE] 7.1 Telemetry capture + admin telemetry endpoints
- [x] [DONE] 7.2 Season calibration persistence (season_calibration)
- [ ] [ALPHA REQUIRED] 7.3 Ensure telemetry is sufficient to calibrate live values (emission, caps, price curves, pressure)
- [ ] [ALPHA REQUIRED] 7.4 Define and verify telemetry events:
  - [x] [DONE] Faucet claims (daily, activity, passive if enabled, login safeguard)
  - [ ] [ALPHA REQUIRED] Star purchase attempts + successes
  - [x] [DONE] Emission pool levels and per‑tick emissions
  - [x] [DONE] Market pressure value per tick
- [ ] [ALPHA REQUIRED] 7.5 Validate append‑only economic logs are complete and queryable
- [ ] [ALPHA REQUIRED] 7.6 Establish calibration workflow using telemetry history (no blind tuning)
- [x] [DONE] 7.7 Reconcile telemetry taxonomy with alpha‑execution.md (current client emits login + buy_star; join_season is not emitted)

---

## Phase 8 — Anti‑Abuse, Trust, and Access Control
- [x] [DONE] 8.1 IP capture + association tracking
- [x] [DONE] 8.2 Enforce one active player per IP per season
- [x] [DONE] 8.3 Soft IP dampening (delay + multipliers)
- [x] [DONE] 8.4 Rate limiting for signup/login
- [x] [DONE] 8.5 AbuseEvents table + signal aggregation
- [ ] [ALPHA REQUIRED] 8.6 Audit anti‑abuse coverage post‑whitelist removal
- [x] [DONE] 8.7 Update anti‑abuse docs to match Alpha (CAPTCHA/verification is post‑alpha)
- [ ] [POST-ALPHA] 8.8 CAPTCHA + verification
- [ ] [POST-ALPHA] 8.9 Additional abuse signals + admin visualization improvements

---

## Phase 9 — Admin & Observability (Read‑Only in Alpha)
- [x] [DONE] 9.1 Admin role + setup key flow
- [x] [DONE] 9.2 Moderator role support
- [x] [DONE] 9.3 Economy monitoring endpoints
- [x] [DONE] 9.4 Notifications system
- [ ] [ALPHA REQUIRED] 9.5 Add notification observability logging
- [ ] [ALPHA REQUIRED] 9.6 Update admin‑tools docs to reflect read‑only Alpha reality
- [ ] [POST-ALPHA] 9.7 Admin safety tools (pause purchases, adjust emission, freeze season)

---

## Phase 10 — Frontend MVP (Alpha)
- [x] [DONE] 10.1 Landing + Auth + Main dashboard + Leaderboard
- [x] [DONE] 10.2 Bulk purchase UI + warnings
- [x] [DONE] 10.3 Admin console entry point
- [ ] [ALPHA REQUIRED] 10.4 Verify UI shows required economy values (price, time, pressure, coins in circulation, next emission)
- [ ] [ALPHA REQUIRED] 10.5 Label missing systems in UI (trading, multi‑season, cosmetics)
- [ ] [POST-ALPHA] 10.6 Season lobby + trading desk + collections + settings/accessibility

---

## Phase 11 — Game Flow & Playability (Explicit and Tested)
- [ ] [ALPHA REQUIRED] 11.1 Map new‑player journey mid‑season (signup → first earn → first star purchase)
- [ ] [ALPHA REQUIRED] 11.2 Map late‑season joiner viability (can play, not necessarily compete)
- [ ] [ALPHA REQUIRED] 11.3 Define always‑available actions vs tightening actions over time
- [ ] [ALPHA REQUIRED] 11.4 Define daily loop steps (login → faucet(s) → purchase) and failure modes
- [ ] [ALPHA REQUIRED] 11.5 Identify any broken/unclear step and add fix tasks before Alpha test

---

## Phase 12 — Season End, Rewards, and Between‑Season Progression
- [x] [DONE] 12.1 End‑of‑season snapshot and economy freeze
- [ ] [POST-ALPHA] 12.2 Reward granting (badges + titles + recognition)
- [ ] [POST-ALPHA] 12.3 Persistent progression + season history + return incentives

---

## Post‑Alpha / Beta — Currency Expansion (Canon Only)
- [ ] [POST-ALPHA] Introduce persistent meta currency (Beta) for cosmetics/identity only; non‑tradable, non‑competitive, season‑persistent.
- [ ] [POST-ALPHA] Implement reward grant logic for persistent meta currency (non‑economic, cosmetic only).
- [ ] [POST-ALPHA] Expose persistent meta currency in UI (cosmetics, titles, badges, collections).
- [ ] [POST-ALPHA] Enforce and document: no conversion paths into Coins or Stars, direct or indirect.
- [ ] [POST-ALPHA] (Post‑Release optional) Influence/reputation metric: non‑spendable, eligibility/visibility‑only, never convertible.

## Post‑Alpha / Beta — Tradable Seasonal Assets (TSAs)
- [ ] [POST-ALPHA] Define TSA canon constraints in code (Beta‑only, seasonal competitive asset, system‑minted only, observable supply, no conversion into Coins/Stars, no minting Coins/Stars).
- [ ] [POST-ALPHA] Implement player‑to‑player TSA trading (negotiated; server enforces legality, caps, and burn; logging; disabled in Alpha).
- [ ] [POST-ALPHA] Implement Star sacrifice → TSA minting (Stars destroyed, immediate rank drop, irreversible).
- [ ] [POST-ALPHA] Add append‑only TSA logs (mint w/ stars_destroyed + source, trade w/ consideration + friction, activation) with admin visibility.
- [ ] [POST-ALPHA] Add TSA season‑end wipe behavior and snapshot/telemetry integration.

---

## Phase 13 — Testing & Validation
- [x] [DONE] 13.1 Simulation engine for pricing + pressure
- [ ] [ALPHA REQUIRED] 13.2 Validate simulation outputs vs live calibration parameters
- [ ] [ALPHA EXECUTION] 13.3 Alpha execution cycle
  - [x] [DONE] Define goals + metrics (README/alpha‑execution.md)
  - [ ] [ALPHA EXECUTION] Recruit testers
  - [ ] [ALPHA EXECUTION] Run 1–2 week test
  - [ ] [ALPHA EXECUTION] Analyze telemetry + prioritize fixes
- [ ] [POST-ALPHA] 13.4 Beta readiness (multi‑season + trading + expanded abuse controls)

---

## Phase 14 — Deployment & Live Ops
- [x] [DONE] 14.1 Fly.io deployment config + migrations
- [x] [DONE] 14.2 Basic monitoring + alerting
- [ ] [POST-ALPHA] 14.3 Backup + restore procedures

---

## Phase 15 — Documentation Alignment (Continuous)
- [x] [DONE] 15.1 Canon README set present
- [ ] [ALPHA REQUIRED] 15.2 Keep docs aligned with code reality after every change

---

## Final Verification Checklist (Maintain Continuously)
- [ ] [ALPHA REQUIRED] No task depends on a later task
- [ ] [ALPHA REQUIRED] All README requirements represented (or explicitly deferred)
- [ ] [ALPHA REQUIRED] Code‑existing features verified (not just implemented)
- [ ] [ALPHA REQUIRED] Path from today → Alpha → Post‑Alpha is continuous

---

## Phase 16 — Population‑Invariant Economy Validation
- [ ] [ALPHA REQUIRED] Audit all population‑coupled inputs (coins in circulation, active coins, market pressure, affordability guardrails) and document effects at 1/5/500 players.
- [ ] [ALPHA REQUIRED] Verify active circulation window (24h) produces stable emission at low population; adjust if faucet starvation occurs.
- [ ] [ALPHA REQUIRED] Add telemetry for activeCoinsInCirculation + activePlayers and confirm visibility in admin telemetry.
- [ ] [ALPHA REQUIRED] Run stress tests: solo season, small group (5–10), large group (500+) using simulation + live tick metrics.
- [ ] [ALPHA REQUIRED] Validate faucet pacing (daily/activity/passive if enabled) against emission pool under low‑population conditions.
- [ ] [ALPHA REQUIRED] Confirm star pricing remains purchasable for a solo player over full season without trivializing scarcity.
