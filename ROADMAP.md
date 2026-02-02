# Too Many Coins — Roadmap

This document tracks the **design and implementation status** of *Too Many Coins*.
Checked items reflect **implemented, tested backend behavior**.
Unchecked items represent planned or in-progress work.

The project follows a principle of **server authority first**, with the frontend acting only as a display and input surface.

---

## 🧱 STEP 1 — Core Concept & Scope (FOUNDATION)

- [x] Define seasonal economy with a hard end
- [x] Define coins as an inflationary currency
- [x] Define stars as a scarce, permanent sink
- [x] Establish time pressure as a core mechanic
- [x] Commit to server-authoritative economy (no client trust)

---

## 🧱 STEP 2 — Backend Shape & Stack

- [x] Go backend
- [x] PostgreSQL persistence
- [x] HTTP-based API (no websockets yet)
- [x] Static frontend served by backend
- [x] Background goroutines for time-based systems

---

## 🧱 STEP 3 — Persistent Global Economy State

- [x] Create `season_economy` table
- [x] Persist global coin pool
- [x] Persist global stars purchased
- [x] Persist emission remainder
- [x] Load economy state on server boot
- [x] Prevent economy reset on restart

---

## 🧱 STEP 4 — Time System

- [x] Define fixed season length (28 days)
- [x] Track seconds remaining server-side
- [x] Use UTC consistently
- [x] Clamp negative time values
- [x] Expose remaining time via `/seasons`

---

## 🧱 STEP 5 — Coin Emission (SUPPLY SIDE)

- [x] Define daily emission target
- [x] Convert emission to per-minute rate
- [x] Emit coins in background tick loop
- [x] Accumulate emission remainder
- [x] Log emission events

---

## 🧱 STEP 6 — Server Authority

- [x] Remove client-side pricing logic
- [x] Remove client-side purchase validation
- [x] Client sends *intent only*
- [x] Server computes prices
- [x] Server validates balances
- [x] Server applies effects
- [x] Server returns authoritative state

---

## 🧱 STEP 7 — Star Pricing Logic

- [x] Move star pricing fully server-side
- [x] Define base star price
- [x] Implement **star scarcity multiplier (primary)**
- [x] Implement **coin inflation multiplier (secondary)**
- [x] Implement **time pressure multiplier (amplifier)**
- [x] Clamp and ceil prices safely
- [x] Return `currentStarPrice` via `/seasons`

---

## 🧱 STEP 8 — UI Reconciliation

- [x] Update price immediately after purchase
- [x] Update coin & star balances without reload
- [x] Handle failure responses cleanly
- [x] Eliminate UI desynchronization
- [x] Ensure UI reflects server truth only

---

## 🧱 STEP 9 — Player Persistence

- [x] Create `players` table
- [x] Persist player coin balances
- [x] Persist player star balances
- [x] Track `created_at`
- [x] Track `last_active_at`
- [x] Load or create players server-side
- [x] Remove starter coin grants

---

## 🧱 STEP 10 — Star Purchases (END-TO-END)

- [x] `/buy-star` endpoint
- [x] Require valid `playerId`
- [x] Reject insufficient funds
- [x] Deduct coins atomically
- [x] Increment player stars
- [x] Increment global stars purchased
- [x] Return authoritative balances

---

## 🧱 STEP 11 — Coin Faucets (ECONOMY CORE)

### Step 11.1–11.3 — Emission Integrity
- [x] Track coins distributed
- [x] Prevent emission overflow
- [x] Enforce monetary conservation

### Step 11.4 — Emission-Capped Distribution
- [x] Implement `TryDistributeCoins`
- [x] Block faucets when emission exhausted
- [x] Centralize distribution gate

### Step 11.5 — Passive Coin Drip
- [x] Passive drip: 1 coin/minute/player
- [x] Server-side scheduler (goroutine)
- [x] Emission-capped
- [x] No client involvement
- [x] Restart-safe behavior

### Step 11.6 — Join-Time Eligibility
- [x] Set `last_active_at = now - 1 minute` on player creation
- [x] No free coins
- [x] No special-case logic
- [x] First drip occurs naturally

---

## 🧱 STEP 12 — Player Identity & Abuse Control (CURRENT)

### Step 12.1 — Browser Identity
- [x] One `playerId` per browser
- [x] Stored in `localStorage`
- [x] Sent with every request

### Step 12.2 — Server Trust Boundary
- [x] Reject malformed or empty `playerId`
- [x] Enforce max length / format
- [x] Restrict player creation to `/player`
- [x] Prevent implicit creation in `/buy-star`

### Step 12.3 — IP Awareness
- [x] Extract IP from `X-Forwarded-For` / `RemoteAddr`
- [x] Log IP ↔ playerId associations
- [x] (Optional) Persist IP on player creation

### Step 12.4 — Soft Limits
- [x] Limit players per IP (e.g. max 2)
- [x] Excess players receive reduced or no drip
- [x] No hard bans

### Step 12.5 — Economic Dampening
- [x] Increase prices for excess players
- [x] Reduce faucet access
- [x] Delay eligibility

---

## 🧱 STEP 13 — Additional Coin Faucets (PLANNED)

- [x] Activity-based rewards
- [x] Risk/reward mechanics
- [x] Time-locked faucets
- [x] Faucet prioritization rules

---

## 🧱 STEP 14 — Additional Sinks (PLANNED)

- [x] New star variants
- [x] Temporary boosts
- [x] Burn mechanics
- [x] Auctions / bidding systems

---

## 🧱 STEP 15 — Season End Logic (PLANNED)

- [x] Define season end conditions
- [x] Handle remaining coins
- [x] Handle stars
- [x] Archive economy state
- [ ] Transition to next season

---

## 🧱 STEP 16 — Balancing & Playtesting

- [ ] Tune emission rates
- [ ] Tune scarcity curves
- [ ] Observe player behavior
- [ ] Adjust multipliers
- [ ] Iterate based on data

### Step 16.A — Alpha Test Readiness (Core)
- [x] Auth: signup/login + session handling
- [x] Auth: profile persistence and restore on login
- [x] Create minimal player profile page
- [x] UI clarity pass (clean, minimal, no over-explaining)
- [x] Add basic onboarding hints (one-time, dismissible)
- [x] Add playtest feature flags (enable/disable faucets, sinks)
- [x] Add test telemetry (session length, faucet usage, purchases)
- [x] Add feedback capture endpoint (survey + freeform)

### Step 16.B — Alpha Test Execution
- [ ] Define Alpha 1 test goals and metrics
- [ ] Recruit 8–15 testers (friends/family ok)
- [ ] Run 1–2 week test cycle
- [ ] Daily/weekly review of economy metrics
- [ ] Compile feedback + prioritized fixes

---

## 🧱 STEP 17 — Multi-Season Support & Lobby

- [ ] Support up to 4 concurrent seasons
- [ ] Stagger season start dates 7 days apart
- [ ] Allow players to join any active season
- [ ] Season lobby view with active seasons
- [ ] Season-specific economy isolation

---

## 🧱 STEP 18 — Daily Resets & Per-Player Caps

- [ ] Daily reset job (UTC) runs once per season-day
- [ ] Reset per-player daily earning totals
- [ ] Refresh daily tasks on reset
- [ ] Enforce per-player daily earning caps
- [ ] Decrease caps as season progresses

---

## 🧱 STEP 19 — Market Pressure & Star Scaling

- [ ] Track market pressure (24h vs 7d rolling averages)
- [ ] Rate-limit pressure changes per tick
- [ ] Apply market pressure multiplicatively to star prices
- [ ] Implement late-season spike in final week
- [ ] Implement non-linear bulk purchase scaling
- [ ] Re-check price & balance at confirmation time
- [ ] Append-only purchase log for every buy

---

## 🧱 STEP 20 — Anti-Abuse, Whitelisting, and Trust

- [ ] Enforce one active player per IP per season by default
- [ ] Block extra accounts unless whitelisted
- [ ] Implement whitelist request flow + wave approvals
- [ ] Rate-limit account creation
- [ ] CAPTCHA + verification
- [ ] Cooldown before new accounts can join a season
- [ ] Per-player and per-IP star purchase throttles
- [ ] Detect clustering behavior and create abuse events
- [ ] Apply reversible throttles based on abuse events

---

## 🧱 STEP 21 — Persistent State Expansion

- [ ] Add account_id and trust_status to players
- [ ] Add whitelist_group_id to players
- [ ] Add seasons table with status and timestamps
- [ ] Add player-season state (coins, stars, daily totals)
- [ ] Append-only coin earning log
- [ ] Append-only star purchase log

---

## 🧱 STEP 22 — Real-Time Updates

- [ ] Server-sent events or WebSockets for season state
- [ ] Broadcast star price updates
- [ ] Broadcast season time remaining
- [ ] Broadcast economy pressure changes

---

## 🧱 STEP 23 — Frontend MVP Pages

- [ ] Landing page
- [ ] Auth/signup/login
- [ ] Season lobby
- [ ] Main season dashboard
- [ ] Bulk purchase UI with cost breakdown + warnings
- [ ] Leaderboard page
- [ ] Player profile + collection page
- [ ] Settings + accessibility
- [ ] Whitelist request page
- [ ] Admin console entry point

---

## 🧱 STEP 24 — Season End & Rewards

- [ ] Freeze economy actions on season end
- [ ] Snapshot final rankings
- [ ] Grant cosmetic rewards by tier
- [ ] Late-season participation recognition
- [ ] Season summary screen and next-season CTA

---

## 🧱 STEP 25 — Between-Season Progression

- [ ] Persistent account level (participation-based)
- [ ] Cosmetics, badges, and titles persistence
- [ ] Season history records on profiles
- [ ] Return incentives (cosmetic-only)
- [ ] Season modifiers that do not alter economy rules

---

## 🧱 STEP 26 — Admin Tools & Moderation

- [ ] Whitelist review dashboard
- [ ] Abuse event review and resolution
- [ ] Economy monitoring (emission, pressure, purchases)
- [ ] Manual throttles and flags

---

## 📜 Version Notes

- **v0.1** — Economy core complete (server-authoritative, persistent, emission-capped)
- **v0.2** — Player persistence and star purchases
- **v0.3** — Identity & abuse controls (in progress)

---

_Last updated: February 2026_
