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

- [ ] New star variants
- [ ] Temporary boosts
- [ ] Burn mechanics
- [ ] Auctions / bidding systems

---

## 🧱 STEP 15 — Season End Logic (PLANNED)

- [ ] Define season end conditions
- [ ] Handle remaining coins
- [ ] Handle stars
- [ ] Archive economy state
- [ ] Transition to next season

---

## 🧱 STEP 16 — Balancing & Playtesting

- [ ] Tune emission rates
- [ ] Tune scarcity curves
- [ ] Observe player behavior
- [ ] Adjust multipliers
- [ ] Iterate based on data

---

## 📜 Version Notes

- **v0.1** — Economy core complete (server-authoritative, persistent, emission-capped)
- **v0.2** — Player persistence and star purchases
- **v0.3** — Identity & abuse controls (in progress)

---

_Last updated: February 2026_
