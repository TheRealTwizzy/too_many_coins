# TODO — Canonical Execution Plan (Single Source of Truth)

This document supersedes any prior task list. It is ordered by dependency and reflects current code + README canon as of February 3, 2026.

Legend: each task is explicitly marked with a status tag.

Status Tags:
- [DONE]
- [ALPHA REQUIRED]
- [POST-ALPHA]

---

## Alpha Exit Criteria (Non-Negotiable)
- [ ] Players can sign up, log in, and enter the active season without manual admin steps (single season is acceptable for Alpha).
- [ ] Economy runs continuously without admin intervention (tick loop, emission, star pricing, market pressure updates).
- [ ] Daily play loop works: daily login faucet + active play faucet + decreasing daily cap + emission pool enforcement.
- [ ] Players can observe the economy clearly in UI/SSE (time remaining, current star price, coins in circulation, market pressure).
- [ ] Star purchase flow works end-to-end (single + bulk), shows warnings, and is atomic.
- [ ] Season end freezes all economy actions and writes end-of-season snapshots.
- [ ] Known missing systems are explicitly labeled in UI/docs (trading, multi-season, cosmetics, etc.).

Allowed to be rough or missing in Alpha:
- Trading system and trading UI
- Multi-season runtime and season lobby
- Cosmetics, badges, titles, and long-term progression
- Profile collections, settings, and accessibility polish
- Admin analytics polish beyond core monitoring

---

## Phase 0 — Inception & Canon Lock
- [x] [DONE] 0.1 Define core game loop and pressure pillars
  - [x] [DONE] Coins → Stars loop with scarcity and inflation pressure
  - [x] [DONE] “No safe move” / psychological pressure principle
- [x] [DONE] 0.2 Define seasonal structure and resets
  - [x] [DONE] 28‑day seasons, staggered start concept, multi-season target
- [x] [DONE] 0.3 Commit to server authority
  - [x] [DONE] Client only sends intent; server computes all economy outcomes
- [x] [DONE] 0.4 Define conditional brokered trading concept
  - [x] [DONE] Coins‑for‑Stars only, system‑priced, burn + premium

---

## Phase 1 — Backend Foundations (Authoritative Core)
- [x] [DONE] 1.1 Establish Go + net/http backend
- [x] [DONE] 1.2 Establish PostgreSQL persistence
- [x] [DONE] 1.3 Create base schema tables
  - [x] [DONE] players, accounts, sessions
  - [x] [DONE] season_economy persistence
  - [x] [DONE] star_purchase_log, notifications, whitelist tables
- [x] [DONE] 1.4 Auth stack with sessions + access/refresh tokens
  - [x] [DONE] Signup/login endpoints with password hashing
  - [x] [DONE] Session cookies + access tokens
- [x] [DONE] 1.5 Server‑side SSE for live snapshots
  - [x] [DONE] Star price + time remaining streaming

---

## Phase 2 — Time System & Season Core
- [x] [DONE] 2.1 Define fixed 28‑day season clock
  - [x] [DONE] Season start/end UTC tracking
  - [x] [DONE] Clamp negative time values
- [x] [DONE] 2.2 Background tick loop
  - [x] [DONE] 60s tick for emission + market pressure
- [x] [DONE] 2.3 Season end snapshot on tick
  - [x] [DONE] Season final rankings + economy snapshot tables
- [ ] [POST-ALPHA] 2.4 Multi‑season runtime model
  - [ ] [POST-ALPHA] Seasons table with status, start/end timestamps
  - [ ] [POST-ALPHA] Per‑season tick scheduling
  - [ ] [POST-ALPHA] Up to 4 concurrent seasons, staggered 7 days
  - [ ] [POST-ALPHA] Allow join any active season
- [x] [DONE] 2.5 Make season selection honest for Alpha (single season)
  - [x] [DONE] /seasons endpoint must return only the real active season (remove hardcoded fake seasons)
  - [x] [DONE] Join state must be server-driven or auto-join (no client-only join state)

---

## Phase 3 — Core Economy Lockdown
- [x] [DONE] 3.1 Emission pool and daily budget
  - [x] [DONE] Emission remainder + per‑tick release
  - [x] [DONE] Emission throttling via pool availability
- [x] [DONE] 3.2 Server‑authoritative star pricing
  - [x] [DONE] Scarcity multiplier (stars purchased)
  - [x] [DONE] Coin inflation multiplier (coins per player)
  - [x] [DONE] Time pressure + late‑season spike
  - [x] [DONE] Market pressure multiplier with caps
  - [x] [DONE] Affordability guardrail
- [x] [DONE] 3.3 Bulk purchase scaling + quote
  - [x] [DONE] Non‑linear quantity multiplier
  - [x] [DONE] Server re‑check at confirmation time
- [x] [DONE] 3.4 Enforce economy authority
  - [x] [DONE] Client sends intent only; server returns truth
  - [x] [DONE] Atomic purchase transactions
- [x] [DONE] 3.5 Strict removal of admin economic control
  - [x] [DONE] Admin economy endpoints read‑only
  - [x] [DONE] Admin player controls block coin/star edits

---

## Phase 4 — Faucets & Sustained Daily Gameplay
- [x] [DONE] 4.1 Daily login faucet
  - [x] [DONE] Cooldown enforcement + faucet claim log
  - [x] [DONE] Emission‑capped distribution
- [x] [DONE] 4.2 Activity faucet
  - [x] [DONE] Activity cooldown + cap enforcement
- [x] [DONE] 4.3 Per‑player daily earning cap that decreases over season
- [ ] [POST-ALPHA] 4.4 Passive active/idle drip
  - [ ] [POST-ALPHA] Active vs idle intervals + throttle behavior
  - [ ] [POST-ALPHA] Drip uses emission pool and daily caps
- [ ] [POST-ALPHA] 4.5 Daily tasks faucet
  - [ ] [POST-ALPHA] Task definitions + refresh at daily reset
  - [ ] [POST-ALPHA] Task completion + capped rewards
- [ ] [POST-ALPHA] 4.6 Comeback reward faucet
  - [ ] [POST-ALPHA] Inactivity eligibility + one‑time reward
- [ ] [ALPHA REQUIRED] 4.7 Append‑only coin earning log
  - [ ] [ALPHA REQUIRED] Log source_type + amount per grant
  - [ ] [ALPHA REQUIRED] Write path from faucets and other coin grants

---

## Phase 5 — Market Pressure (Pressure, Not Relief)
- [x] [DONE] 5.1 Market pressure computed server‑side
  - [x] [DONE] 24h vs 7d rolling averages
  - [x] [DONE] Rate‑limited adjustments per tick
- [ ] [ALPHA REQUIRED] 5.2 Pressure broadcast to clients
  - [ ] [ALPHA REQUIRED] Include pressure in SSE snapshot payload
  - [ ] [ALPHA REQUIRED] UI display of current pressure

---

## Phase 6 — Conditional Brokered Trading System
- [ ] [POST-ALPHA] 6.1 Trade eligibility gates
  - [ ] [POST-ALPHA] Active participation checks
  - [ ] [POST-ALPHA] Recent spend requirement
  - [ ] [POST-ALPHA] Tightening ratios (stars, liquidity, inflation exposure)
- [ ] [POST-ALPHA] 6.2 Brokered trade quoting
  - [ ] [POST-ALPHA] System‑set price ≥ current star price
  - [ ] [POST-ALPHA] Time‑based premium curve
  - [ ] [POST-ALPHA] Coin burn overhead + asymmetric payout
- [ ] [POST-ALPHA] 6.3 Trade limits by season time
  - [ ] [POST-ALPHA] Max stars per trade decreases over time
  - [ ] [POST-ALPHA] Daily trade limits decrease over time
- [ ] [POST-ALPHA] 6.4 Trade execution and logging
  - [ ] [POST-ALPHA] Atomic trade transaction with burn
  - [ ] [POST-ALPHA] Append‑only trade log table
- [ ] [POST-ALPHA] 6.5 Trading desk UI
  - [ ] [POST-ALPHA] Eligibility + cost disclosure
  - [ ] [POST-ALPHA] Confirmation warnings

---

## Phase 7 — Anti‑Abuse, Trust, and Access Control
- [x] [DONE] 7.1 IP capture + association tracking
- [x] [DONE] 7.2 Whitelist request flow + approvals
  - [x] [DONE] Request submission endpoint
  - [x] [DONE] Admin review + approve/deny
- [ ] [ALPHA REQUIRED] 7.3 Enforce one active player per IP per season
  - [ ] [ALPHA REQUIRED] Enforce 1 account/IP by default for season participation (not just signup)
  - [ ] [ALPHA REQUIRED] Respect whitelist max_accounts for exceptions
  - [ ] [ALPHA REQUIRED] Remove hardcoded maxPlayersPerIP=2 bypass in faucet gating
- [x] [DONE] 7.4 Soft IP dampening
  - [x] [DONE] Delay + price multiplier for excess accounts
- [ ] [ALPHA REQUIRED] 7.5 Account creation protections (minimum viable)
  - [ ] [ALPHA REQUIRED] Rate limiting for signup/login endpoints
  - [ ] [ALPHA REQUIRED] Cooldown before new accounts can join a season
  - [ ] [POST-ALPHA] CAPTCHA + verification
- [ ] [POST-ALPHA] 7.6 Abuse detection + events
  - [ ] [POST-ALPHA] Clustering detection
  - [ ] [POST-ALPHA] AbuseEvents table + logging
  - [ ] [POST-ALPHA] Reversible throttles
- [ ] [POST-ALPHA] 7.7 Star purchase throttles
  - [ ] [POST-ALPHA] Per‑player limits
  - [ ] [POST-ALPHA] Per‑IP limits

---

## Phase 8 — Role‑Based Moderation & Admin Tools
- [x] [DONE] 8.1 Admin role + setup key flow
- [x] [DONE] 8.2 Moderator role support
  - [x] [DONE] Moderator profile editing endpoint
- [x] [DONE] 8.3 Economy monitoring dashboard endpoints
- [x] [DONE] 8.4 Telemetry capture + admin telemetry
- [x] [DONE] 8.5 Whitelist management dashboard
- [ ] [POST-ALPHA] 8.6 Abuse event review + resolution UI
- [ ] [POST-ALPHA] 8.7 Trade monitoring for admins
  - [ ] [POST-ALPHA] Trade volume + burn metrics

---

## Phase 9 — Persistence Expansion & Multi‑Season Data Model
- [ ] [POST-ALPHA] 9.1 Seasons table + status transitions
- [ ] [POST-ALPHA] 9.2 PlayerSeasonState table
  - [ ] [POST-ALPHA] Per‑season coins, stars, daily totals
- [ ] [POST-ALPHA] 9.3 Trust status + whitelist group on players
- [x] [DONE] 9.4 Season calibration persistence
  - [x] [DONE] season_calibration table + load/save
- [ ] [ALPHA REQUIRED] 9.5 Append‑only coin earning log (duplicate of 4.7)
  - [ ] [ALPHA REQUIRED] Schema + write path from faucets

---

## Phase 10 — Frontend MVP Completion
- [x] [DONE] 10.1 Landing page
- [x] [DONE] 10.2 Auth/signup/login page
- [x] [DONE] 10.3 Main season dashboard
- [x] [DONE] 10.4 Bulk purchase UI + warnings
- [x] [DONE] 10.5 Leaderboard page
- [x] [DONE] 10.6 Whitelist request page
- [x] [DONE] 10.7 Admin console entry point
- [ ] [POST-ALPHA] 10.8 Season lobby
- [ ] [POST-ALPHA] 10.9 Player profile + collection page
  - [x] [DONE] Profile editing exists
  - [ ] [POST-ALPHA] Collections / cosmetics view
- [ ] [POST-ALPHA] 10.10 Settings + accessibility
- [ ] [POST-ALPHA] 10.11 Trading desk UI
- [ ] [ALPHA REQUIRED] 10.12 Pressure‑focused UI cues
  - [ ] [ALPHA REQUIRED] Show late‑season scarcity messaging
  - [ ] [ALPHA REQUIRED] Display market pressure in the main HUD
  - [ ] [POST-ALPHA] Display trade premium burn warnings

---

## Phase 11 — Season End, Rewards, and Between‑Season Progression
- [x] [DONE] 11.1 End‑of‑season economy snapshot
  - [x] [DONE] Snapshot rankings + economy state
- [x] [DONE] 11.2 Freeze all economy actions on season end
  - [x] [DONE] Star purchases, faucets, burns, and boosts reject after end
- [ ] [POST-ALPHA] 11.3 Reward granting
  - [ ] [POST-ALPHA] Badges + titles by tier
  - [ ] [POST-ALPHA] Late‑season participation recognition
- [ ] [POST-ALPHA] 11.4 Season summary screen + next‑season CTA
- [ ] [POST-ALPHA] 11.5 Persistent progression
  - [ ] [POST-ALPHA] Account level (participation‑based)
  - [ ] [POST-ALPHA] Cosmetic collections + titles + badges
  - [ ] [POST-ALPHA] Season history records
  - [ ] [POST-ALPHA] Return incentives (cosmetic only)

---

## Phase 12 — Testing & Validation
- [x] [DONE] 12.1 Simulation engine for pricing + pressure
- [x] [DONE] 12.2 Alpha readiness checklist
- [ ] [ALPHA REQUIRED] 12.3 Alpha execution cycle
  - [ ] [ALPHA REQUIRED] Define goals + metrics
  - [ ] [ALPHA REQUIRED] Recruit testers
  - [ ] [ALPHA REQUIRED] Run 1–2 week test
  - [ ] [ALPHA REQUIRED] Analyze telemetry + fix priorities
- [ ] [POST-ALPHA] 12.4 Beta readiness
  - [ ] [POST-ALPHA] Multi‑season support verified
  - [ ] [POST-ALPHA] Trading system verified
  - [ ] [POST-ALPHA] Anti‑abuse expansions verified

---

## Phase 13 — Deployment & Live Ops
- [ ] [ALPHA REQUIRED] 13.1 Production environment configuration
  - [ ] [ALPHA REQUIRED] Railway deployment config
  - [ ] [ALPHA REQUIRED] DB migration strategy
- [ ] [ALPHA REQUIRED] 13.2 Monitoring + alerting (minimum viable)
  - [ ] [ALPHA REQUIRED] Economy integrity dashboards
  - [ ] [ALPHA REQUIRED] Error/uptime alerting
- [ ] [POST-ALPHA] 13.3 Backup + restore procedures

---

## Phase 14 — Documentation Alignment
- [x] [DONE] 14.1 Canon README set present
- [ ] [ALPHA REQUIRED] 14.2 Document bot runner usage in primary README
- [ ] [ALPHA REQUIRED] 14.3 Document notifications + password reset flows
- [ ] [ALPHA REQUIRED] 14.4 Document admin/role workflows
- [ ] [ALPHA REQUIRED] 14.5 Resolve README contradictions vs code (Alpha honesty)
  - [ ] [ALPHA REQUIRED] Main README multi‑season claims vs README/first-playable single‑season reality
  - [ ] [ALPHA REQUIRED] Admin knobs in README vs enforced removal of admin economic control
  - [ ] [ALPHA REQUIRED] Trading described as core vs no trading in code
  - [ ] [ALPHA REQUIRED] Anti‑abuse guarantees vs missing rate limits/throttles
  - [ ] [ALPHA REQUIRED] Passive drip claim vs current active‑only faucet
  - [ ] [ALPHA REQUIRED] Market pressure inputs mention trades not implemented

---

## Phase 15 — Post‑Launch & Seasonal Continuity
- [ ] [POST-ALPHA] 15.1 Season rollover automation
  - [ ] [POST-ALPHA] Create next season records on schedule
  - [ ] [POST-ALPHA] Archive ended seasons
- [ ] [POST-ALPHA] 15.2 Long‑term integrity monitoring
  - [ ] [POST-ALPHA] Track coin supply vs burn vs faucet output
  - [ ] [POST-ALPHA] Detect long‑term liquidity collapse
- [ ] [POST-ALPHA] 15.3 Season history archive UX

---

## Final Verification Checklist (Maintain Continuously)
- [ ] [ALPHA REQUIRED] No task depends on a later task
- [ ] [ALPHA REQUIRED] All README requirements represented (or explicitly deferred)
- [ ] [ALPHA REQUIRED] Code‑existing features documented
- [ ] [ALPHA REQUIRED] Path from today → live → post‑launch is continuous
