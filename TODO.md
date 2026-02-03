# TODO — Canonical Execution Plan (Single Source of Truth)

This document supersedes any prior task list. It is ordered by dependency and reflects current code + README canon as of February 3, 2026.

Legend: each task is explicitly marked as DONE or NOT STARTED.

---

## Phase 0 — Inception & Canon Lock
- [x] 0.1 Define core game loop and pressure pillars (DONE)
  - [x] Coins → Stars loop with scarcity and inflation pressure (DONE)
  - [x] “No safe move” / psychological pressure principle (DONE)
- [x] 0.2 Define seasonal structure and resets (DONE)
  - [x] 28‑day seasons, staggered start concept, multi-season target (DONE)
- [x] 0.3 Commit to server authority (DONE)
  - [x] Client only sends intent; server computes all economy outcomes (DONE)
- [x] 0.4 Define conditional brokered trading concept (DONE)
  - [x] Coins‑for‑Stars only, system‑priced, burn + premium (DONE)

---

## Phase 1 — Backend Foundations (Authoritative Core)
- [x] 1.1 Establish Go + net/http backend (DONE)
- [x] 1.2 Establish PostgreSQL persistence (DONE)
- [x] 1.3 Create base schema tables (DONE)
  - [x] players, accounts, sessions (DONE)
  - [x] season_economy persistence (DONE)
  - [x] star_purchase_log, notifications, whitelist tables (DONE)
- [x] 1.4 Auth stack with sessions + access/refresh tokens (DONE)
  - [x] Signup/login endpoints with password hashing (DONE)
  - [x] Session cookies + access tokens (DONE)
- [x] 1.5 Server‑side SSE for live snapshots (DONE)
  - [x] Star price + time remaining streaming (DONE)

---

## Phase 2 — Time System & Season Core
- [x] 2.1 Define fixed 28‑day season clock (DONE)
  - [x] Season start/end UTC tracking (DONE)
  - [x] Clamp negative time values (DONE)
- [x] 2.2 Background tick loop (DONE)
  - [x] 60s tick for emission + market pressure (DONE)
- [x] 2.3 Season end snapshot on tick (DONE)
  - [x] Season final rankings + economy snapshot tables (DONE)
- [ ] 2.4 Multi‑season runtime model (NOT STARTED)
  - [ ] Seasons table with status, start/end timestamps (NOT STARTED)
  - [ ] Per‑season tick scheduling (NOT STARTED)
  - [ ] Up to 4 concurrent seasons, staggered 7 days (NOT STARTED)
  - [ ] Allow join any active season (NOT STARTED)

---

## Phase 3 — Core Economy Lockdown
- [x] 3.1 Emission pool and daily budget (DONE)
  - [x] Emission remainder + per‑tick release (DONE)
  - [x] Emission throttling via pool availability (DONE)
- [x] 3.2 Server‑authoritative star pricing (DONE)
  - [x] Scarcity multiplier (stars purchased) (DONE)
  - [x] Coin inflation multiplier (coins per player) (DONE)
  - [x] Time pressure + late‑season spike (DONE)
  - [x] Market pressure multiplier with caps (DONE)
  - [x] Affordability guardrail (DONE)
- [x] 3.3 Bulk purchase scaling + quote (DONE)
  - [x] Non‑linear quantity multiplier (DONE)
  - [x] Server re‑check at confirmation time (DONE)
- [x] 3.4 Enforce economy authority (DONE)
  - [x] Client sends intent only; server returns truth (DONE)
  - [x] Atomic purchase transactions (DONE)
- [x] 3.5 Strict removal of admin economic control (DONE)
  - [x] Admin economy endpoints read‑only (DONE)
  - [x] Admin player controls block coin/star edits (DONE)

---

## Phase 4 — Faucets & Sustained Daily Gameplay
- [x] 4.1 Daily login faucet (DONE)
  - [x] Cooldown enforcement + faucet claim log (DONE)
  - [x] Emission‑capped distribution (DONE)
- [x] 4.2 Activity faucet (DONE)
  - [x] Activity cooldown + cap enforcement (DONE)
- [x] 4.3 Per‑player daily earning cap that decreases over season (DONE)
- [ ] 4.4 Passive active/idle drip (NOT STARTED)
  - [ ] Active vs idle intervals + throttle behavior (NOT STARTED)
  - [ ] Drip uses emission pool and daily caps (NOT STARTED)
- [ ] 4.5 Daily tasks faucet (NOT STARTED)
  - [ ] Task definitions + refresh at daily reset (NOT STARTED)
  - [ ] Task completion + capped rewards (NOT STARTED)
- [ ] 4.6 Comeback reward faucet (NOT STARTED)
  - [ ] Inactivity eligibility + one‑time reward (NOT STARTED)
- [ ] 4.7 Append‑only coin earning log (NOT STARTED)
  - [ ] Log source_type + amount per grant (NOT STARTED)

---

## Phase 5 — Market Pressure (Pressure, Not Relief)
- [x] 5.1 Market pressure computed server‑side (DONE)
  - [x] 24h vs 7d rolling averages (DONE)
  - [x] Rate‑limited adjustments per tick (DONE)
- [ ] 5.2 Pressure broadcast to clients (NOT STARTED)
  - [ ] Include pressure in SSE snapshot payload (NOT STARTED)
  - [ ] UI display of current pressure (NOT STARTED)

---

## Phase 6 — Conditional Brokered Trading System
- [ ] 6.1 Trade eligibility gates (NOT STARTED)
  - [ ] Active participation checks (NOT STARTED)
  - [ ] Recent spend requirement (NOT STARTED)
  - [ ] Tightening ratios (stars, liquidity, inflation exposure) (NOT STARTED)
- [ ] 6.2 Brokered trade quoting (NOT STARTED)
  - [ ] System‑set price ≥ current star price (NOT STARTED)
  - [ ] Time‑based premium curve (NOT STARTED)
  - [ ] Coin burn overhead + asymmetric payout (NOT STARTED)
- [ ] 6.3 Trade limits by season time (NOT STARTED)
  - [ ] Max stars per trade decreases over time (NOT STARTED)
  - [ ] Daily trade limits decrease over time (NOT STARTED)
- [ ] 6.4 Trade execution and logging (NOT STARTED)
  - [ ] Atomic trade transaction with burn (NOT STARTED)
  - [ ] Append‑only trade log table (NOT STARTED)
- [ ] 6.5 Trading desk UI (NOT STARTED)
  - [ ] Eligibility + cost disclosure (NOT STARTED)
  - [ ] Confirmation warnings (NOT STARTED)

---

## Phase 7 — Anti‑Abuse, Trust, and Access Control
- [x] 7.1 IP capture + association tracking (DONE)
- [x] 7.2 Whitelist request flow + approvals (DONE)
  - [x] Request submission endpoint (DONE)
  - [x] Admin review + approve/deny (DONE)
- [ ] 7.3 Enforce one active player per IP per season (NOT STARTED)
  - [ ] Hard block extra accounts unless whitelisted (NOT STARTED)
- [x] 7.4 Soft IP dampening (DONE)
  - [x] Delay + price multiplier for excess accounts (DONE)
- [ ] 7.5 Account creation protections (NOT STARTED)
  - [ ] Rate limiting (NOT STARTED)
  - [ ] CAPTCHA + verification (NOT STARTED)
  - [ ] Cooldown before joining a season (NOT STARTED)
- [ ] 7.6 Abuse detection + events (NOT STARTED)
  - [ ] Clustering detection (NOT STARTED)
  - [ ] AbuseEvents table + logging (NOT STARTED)
  - [ ] Reversible throttles (NOT STARTED)
- [ ] 7.7 Star purchase throttles (NOT STARTED)
  - [ ] Per‑player limits (NOT STARTED)
  - [ ] Per‑IP limits (NOT STARTED)

---

## Phase 8 — Role‑Based Moderation & Admin Tools
- [x] 8.1 Admin role + setup key flow (DONE)
- [x] 8.2 Moderator role support (DONE)
  - [x] Moderator profile editing endpoint (DONE)
- [x] 8.3 Economy monitoring dashboard endpoints (DONE)
- [x] 8.4 Telemetry capture + admin telemetry (DONE)
- [x] 8.5 Whitelist management dashboard (DONE)
- [ ] 8.6 Abuse event review + resolution UI (NOT STARTED)
- [ ] 8.7 Trade monitoring for admins (NOT STARTED)
  - [ ] Trade volume + burn metrics (NOT STARTED)

---

## Phase 9 — Persistence Expansion & Multi‑Season Data Model
- [ ] 9.1 Seasons table + status transitions (NOT STARTED)
- [ ] 9.2 PlayerSeasonState table (NOT STARTED)
  - [ ] Per‑season coins, stars, daily totals (NOT STARTED)
- [ ] 9.3 Trust status + whitelist group on players (NOT STARTED)
- [ ] 9.4 Season calibration persistence (NOT STARTED)
  - [ ] season_calibration table + load/save (NOT STARTED)
- [ ] 9.5 Append‑only coin earning log (NOT STARTED)
  - [ ] Schema + write path from faucets (NOT STARTED)

---

## Phase 10 — Frontend MVP Completion
- [x] 10.1 Landing page (DONE)
- [x] 10.2 Auth/signup/login page (DONE)
- [x] 10.3 Main season dashboard (DONE)
- [x] 10.4 Bulk purchase UI + warnings (DONE)
- [x] 10.5 Leaderboard page (DONE)
- [x] 10.6 Whitelist request page (DONE)
- [x] 10.7 Admin console entry point (DONE)
- [ ] 10.8 Season lobby (NOT STARTED)
- [ ] 10.9 Player profile + collection page (NOT STARTED)
- [ ] 10.10 Settings + accessibility (NOT STARTED)
- [ ] 10.11 Trading desk UI (NOT STARTED)
- [ ] 10.12 Pressure‑focused UI cues (NOT STARTED)
  - [ ] Show late‑season scarcity messaging (NOT STARTED)
  - [ ] Display market pressure + trade premium burn warnings (NOT STARTED)

---

## Phase 11 — Season End, Rewards, and Between‑Season Progression
- [x] 11.1 End‑of‑season economy snapshot (DONE)
  - [x] Snapshot rankings + economy state (DONE)
- [ ] 11.2 Freeze all economy actions on season end (NOT STARTED)
  - [ ] Disable trading, faucets, purchases for ended seasons (NOT STARTED)
- [ ] 11.3 Reward granting (NOT STARTED)
  - [ ] Badges + titles by tier (NOT STARTED)
  - [ ] Late‑season participation recognition (NOT STARTED)
- [ ] 11.4 Season summary screen + next‑season CTA (NOT STARTED)
- [ ] 11.5 Persistent progression (NOT STARTED)
  - [ ] Account level (participation‑based) (NOT STARTED)
  - [ ] Cosmetic collections + titles + badges (NOT STARTED)
  - [ ] Season history records (NOT STARTED)
  - [ ] Return incentives (cosmetic only) (NOT STARTED)

---

## Phase 12 — Testing & Validation
- [x] 12.1 Simulation engine for pricing + pressure (DONE)
- [x] 12.2 Alpha readiness checklist (DONE)
- [ ] 12.3 Alpha execution cycle (NOT STARTED)
  - [ ] Define goals + metrics (NOT STARTED)
  - [ ] Recruit testers (NOT STARTED)
  - [ ] Run 1–2 week test (NOT STARTED)
  - [ ] Analyze telemetry + fix priorities (NOT STARTED)
- [ ] 12.4 Beta readiness (NOT STARTED)
  - [ ] Multi‑season support verified (NOT STARTED)
  - [ ] Trading system verified (NOT STARTED)
  - [ ] Anti‑abuse expansions verified (NOT STARTED)

---

## Phase 13 — Deployment & Live Ops
- [ ] 13.1 Production environment configuration (NOT STARTED)
  - [ ] Railway deployment config (NOT STARTED)
  - [ ] DB migration strategy (NOT STARTED)
- [ ] 13.2 Monitoring + alerting (NOT STARTED)
  - [ ] Economy integrity dashboards (NOT STARTED)
  - [ ] Abuse detection alerts (NOT STARTED)
- [ ] 13.3 Backup + restore procedures (NOT STARTED)

---

## Phase 14 — Documentation Alignment
- [x] 14.1 Canon README set present (DONE)
- [ ] 14.2 Document bot runner usage in primary README (NOT STARTED)
- [ ] 14.3 Document notifications + password reset flows (NOT STARTED)
- [ ] 14.4 Document admin/role workflows (NOT STARTED)

---

## Phase 15 — Post‑Launch & Seasonal Continuity
- [ ] 15.1 Season rollover automation (NOT STARTED)
  - [ ] Create next season records on schedule (NOT STARTED)
  - [ ] Archive ended seasons (NOT STARTED)
- [ ] 15.2 Long‑term integrity monitoring (NOT STARTED)
  - [ ] Track coin supply vs burn vs faucet output (NOT STARTED)
  - [ ] Detect long‑term liquidity collapse (NOT STARTED)
- [ ] 15.3 Season history archive UX (NOT STARTED)

---

## Final Verification Checklist (Maintain Continuously)
- [ ] No task depends on a later task (NOT STARTED)
- [ ] All README requirements represented (NOT STARTED)
- [ ] Code‑existing features documented (NOT STARTED)
- [ ] Path from today → live → post‑launch is continuous (NOT STARTED)
