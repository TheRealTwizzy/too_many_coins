# TODO — Canonical Execution Plan (Single Source of Truth)

This document supersedes any prior task list. It is ordered by dependency and reflects current code + README canon as of February 7, 2026.

Legend: each task is explicitly marked with a status tag.

Status Tags:
- [DONE]
- [ALPHA REQUIRED]
- [ALPHA EXECUTION]
- [POST-ALPHA]
- [POST-BETA]

---

## Alpha Exit Criteria (Non‑Negotiable)
- [ ] [ALPHA REQUIRED] Players can sign up, log in, and enter the active season without manual admin steps (single season is acceptable for Alpha).
- [ ] [ALPHA REQUIRED] Universal Basic Income (UBI) provides minimum 0.001 coin per tick to all players (economy foundation).
- [ ] [ALPHA REQUIRED] Economy runs continuously without admin intervention (tick loop, emission, UBI, star pricing, market pressure updates).
- [ ] [ALPHA REQUIRED] Daily play loop works: UBI + daily login faucet + active play faucet + decreasing daily cap + emission pool enforcement.
- [ ] [ALPHA REQUIRED] Login playability safeguard keeps new/returning players playable within minutes (alpha‑only, emission‑pool backed).
- [ ] [ALPHA REQUIRED] Players can observe the economy clearly in UI/SSE (time remaining, current star price, coins in circulation, market pressure, emission cadence).
- [ ] [ALPHA REQUIRED] Star purchase flow works end‑to‑end (single + bulk), shows warnings, and is atomic.
- [ ] [ALPHA REQUIRED] Season end freezes all economy actions and writes end‑of‑season snapshots.
- [ ] [ALPHA REQUIRED] Known missing systems are explicitly labeled in UI/docs (trading, multi‑season, cosmetics, communication, rare currencies, etc.).

Allowed to be rough or missing in Alpha:
- Brokered trading (Coins ↔ Stars) and trading UI
- Player‑to‑player trading (TSAs, rare currencies)
- Multi‑season runtime and season lobby
- Cosmetics, badges, titles, and long‑term progression (persistent meta currency, profile collections)
- Direct messaging and forum systems
- Rare currencies (Beta/Release)
- Settings and accessibility polish
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
- [x] [DONE] 0.8 Reconcile TODO.md with Game Bible (February 2026 audit; this document completed)

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
- [x] [DONE] 2.4a Expose server day index + total days to UI (no client hardcoding)
- [x] [DONE] 2.4b Alpha season persistence + auto‑advance (single‑season, restart‑safe)
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
- [x] [DONE] 4.0 Universal Basic Income (UBI) Implementation
  - [x] [DONE] 4.0a Implement minimum 0.001 coin per tick payout to all active players
  - [x] [DONE] 4.0b Verify UBI draws from emission pool and respects pool throttling
  - [x] [DONE] 4.0c Confirm UBI is foundation (all other faucets are additive)
  - [ ] [ALPHA REQUIRED] 4.0d Document UBI in coin-faucets.md if not already present
  - [ ] [ALPHA REQUIRED] 4.0e Ensure star pricing tuning accounts for UBI + inflation interaction
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
  - [x] [DONE] Star price persistence (current_star_price in season_economy)
  - [x] [DONE] 5.1a Enforce season-authoritative (player-divergent-free) pricing — star price computed once per tick, shared identically across all players, uses only season-level inputs (no active player metrics)
- [x] [DONE] 5.2 Atomic star purchases (single + bulk)
- [x] [DONE] 5.2a Align pricing time progression to runtime season length (Alpha 14 days / extension-aware)
- [ ] [ALPHA REQUIRED] 5.3 Validate pricing curves vs coin emission (affordability and late‑season scarcity)
- [x] [DONE] 5.4 Validate bulk purchase warnings and re‑check at confirmation

---

## Phase 6 — Market Pressure
- [x] [DONE] 6.1 Market pressure computed server‑side
- [x] [DONE] 6.2 Rate‑limited adjustments per tick
- [x] [DONE] 6.3 Validate pressure inputs vs canon (no trade inputs until trading exists)
- [x] [DONE] 6.4 Validate pressure appears in SSE + UI and is stable under bursts

---

## Phase 7 — Telemetry, Calibration, and Economic History (Data‑Driven Only)
- [x] [DONE] 7.1 Telemetry capture + admin telemetry endpoints
- [x] [DONE] 7.2 Season calibration persistence (season_calibration)
- [x] [DONE] 7.3 Ensure telemetry is sufficient to calibrate live values (emission, caps, price curves, pressure)
- [ ] [ALPHA REQUIRED] 7.4 Define and verify telemetry events:
  - [x] [DONE] Faucet claims (daily, activity, passive if enabled, login safeguard)
  - [x] [DONE] Star purchase attempts + successes
  - [x] [DONE] Emission pool levels and per‑tick emissions
  - [x] [DONE] Market pressure value per tick
- [x] [DONE] 7.5 Validate append‑only economic logs are complete and queryable
- [x] [DONE] 7.6 Establish calibration workflow using telemetry history (no blind tuning)
- [x] [DONE] 7.7 Reconcile telemetry taxonomy with alpha‑execution.md (current client emits login + buy_star; join_season is not emitted)

---

## Phase 8 — Anti‑Abuse, Trust, and Access Control (Soft Enforcement Philosophy)

### Anti-Cheat Philosophy: Sentinels, Not Punishers

Anti-cheat is **gradual, invisible, and corrective**, not punitive.

**What anti-cheat NEVER does:**
- Bans automatically
- Suspends accounts automatically
- Zeroes wallets
- Hard-blocks players
- Exposes enforcement actions publicly

**What anti-cheat DOES:**
- Gradually reduces earning rates for suspicious behavior
- Increases star prices for suspicious accounts
- Adds cooldowns and jitter to sensitive actions
- Throttles activity without blocking it

**Goal: Make abuse economically ineffective, not publicly punishing.**

### Implementation Status

- [x] [DONE] 8.1 IP capture + association tracking
- [x] [DONE] 8.2 Enforce one active player per IP per season (soft enforcement: throttles, not hard blocks)
- [x] [DONE] 8.3 Soft IP dampening (delay + multipliers)
- [x] [DONE] 8.4 Rate limiting for signup/login
- [x] [DONE] 8.5 AbuseEvents table + signal aggregation
- [x] [DONE] 8.6 Audit anti‑abuse coverage post‑whitelist removal
- [x] [DONE] 8.7 Update anti‑abuse docs to match Alpha (CAPTCHA/verification is post‑alpha)
- [ ] [ALPHA REQUIRED] 8.8 Verify soft enforcement scaling: minor suspicious activity → minor throttles; extreme abuse → heavy dampening
- [ ] [ALPHA REQUIRED] 8.9 Confirm admin ban capability exists ONLY for extreme cases flagged by anti-cheat
- [ ] [POST-ALPHA] 8.10 CAPTCHA + verification
- [ ] [POST-ALPHA] 8.11 Additional abuse signals + admin visualization improvements
- [ ] [POST-ALPHA] 8.12 Trade-specific abuse detection (reciprocal trades, IP clustering, volume spikes)

---

## Phase 9 — Admin & Observability (Read‑Only in Alpha; Sentinels, Not Gods)

### Admin Role Philosophy: Sentinels, Not Gods

**Admins are sentinels, not gods.** The economy must self-regulate. Admins provide oversight and emergency safeguards, not active management.

**What admins MAY do:**
- Emergency pause 1 or all seasons (temporary freeze)
- Ban extreme abuse cases (only after anti-cheat recommendation)
- Monitor telemetry and economy health (read-only observability)
- Advance seasons manually (recovery only, not normal flow)

**What admins MUST NOT do:**
- Micromanage the economy
- Manually adjust player balances
- Override anti-cheat without justification
- Edit past season data
- Interfere with normal economic flow

**The economy is designed to self-regulate. Admin intervention must be rare, deliberate, and auditable.**

### Implementation Status

- [x] [DONE] 9.1 Alpha admin bootstrap (owner claim code, one‑shot, DB‑sealed)
- [x] [DONE] 9.2 Moderator role support
- [x] [DONE] 9.3 Economy monitoring endpoints (read-only)
- [x] [DONE] 9.4 Notifications system
- [x] [DONE] 9.5 Add notification observability logging
- [x] [DONE] 9.6 Update admin‑tools docs to reflect read‑only Alpha reality
- [ ] [ALPHA REQUIRED] 9.6a Verify notification emission, persistence, and client rendering end‑to‑end
- [ ] [ALPHA REQUIRED] 9.7 Verify admin UI clearly communicates "read-only" status in Alpha
- [ ] [ALPHA REQUIRED] 9.8 Confirm admin manual season advance exists for recovery (POST /admin/seasons/advance)
- [ ] [POST-ALPHA] 9.9 Admin safety tools (pause purchases, adjust emission, freeze season)
- [ ] [POST-ALPHA] 9.9a Admin-triggered broadcast notifications
- [ ] [POST-ALPHA] 9.9b Targeted notifications to individual players
- [ ] [POST-ALPHA] 9.10 Trading visibility (premium, burn rate, eligibility tightness, trade logs)
- [ ] [POST-ALPHA] 9.11 Enhanced player inspection (coin earning history, throttle status detail, IP clustering views)

Alpha admin bootstrap finalized: owner claim code, one‑shot, DB‑sealed. No gate keys.

---

## Phase 10 — Frontend MVP (Alpha)
- [x] [DONE] 10.1 Landing + Auth + Main dashboard + Leaderboard
- [x] [DONE] 10.2 Bulk purchase UI + warnings
- [x] [DONE] 10.3 Admin console entry point
- [x] [DONE] 10.4 Verify UI shows required economy values (price, time, pressure, coins in circulation, next emission)
- [x] [DONE] 10.5 Label missing systems in UI (trading, multi‑season, cosmetics)
- [x] [DONE] 10.5a Season end UI consistency (Ended only; no buy/earn; frozen metrics)
- [ ] [ALPHA REQUIRED] 10.6 Update UI labels to include missing communication systems (direct messaging, forum) and rare currencies
- [ ] [POST-ALPHA] 10.7 Season lobby + brokered trading desk (Coins ↔ Stars)
- [ ] [POST-ALPHA] 10.8 Player profile + collections + settings/accessibility
- [ ] [POST-BETA] 10.9 Player-to-player trading desk (TSAs, rare currencies)
- [ ] [POST-BETA] 10.10 Direct messaging UI
- [ ] [POST-BETA] 10.11 Forum UI

---

## Phase 11 — Game Flow & Playability (Explicit and Tested)
- [x] [DONE] 11.1 Map new‑player journey mid‑season (signup → first earn → first star purchase)
- [x] [DONE] 11.2 Map late‑season joiner viability (can play, not necessarily compete)
- [x] [DONE] 11.3 Define always‑available actions vs tightening actions over time
- [x] [DONE] 11.4 Define daily loop steps (login → faucet(s) → purchase) and failure modes
- [x] [DONE] 11.5 Identify any broken/unclear step and add fix tasks before Alpha test
  - [x] [DONE] Add market pressure + next emission to /seasons response to prevent UI gaps before SSE connects

---

## Phase 12 — Season End, Star Conversion, and Between‑Season Progression
- [x] [DONE] 12.1 End‑of‑season snapshot and economy freeze
- [x] [DONE] 12.1a Expose a single terminal season state to clients (Ended only; "Ending" internal)
- [x] [DONE] 12.1b Season lifecycle integrity: Alpha length guardrails + ended invariants + final snapshot fields
- [ ] [ALPHA REQUIRED] 12.2 Star Conversion to Permanent Profile Statistic
  - [ ] [ALPHA REQUIRED] 12.2a At season end, Stars convert to permanent, non-spendable profile statistic (NOT a currency)
  - [ ] [ALPHA REQUIRED] 12.2b Stars are NOT tradable (before or after conversion)
  - [ ] [ALPHA REQUIRED] 12.2c Stars are NOT spendable (except via Star Sacrifice for TSAs during season in Beta+)
  - [ ] [ALPHA REQUIRED] 12.2d Star value scales with season population (larger/more competitive seasons carry more weight)
  - [ ] [ALPHA REQUIRED] 12.2e Document: "Stars are the permanent score of seasonal performance"
- [ ] [POST-ALPHA] 12.3 Reward granting (badges + titles + recognition)
- [ ] [POST-ALPHA] 12.4 Persistent meta currency grant (cosmetic/identity only; never tradable, never converts to Coins/Stars)
- [ ] [POST-ALPHA] 12.5 Persistent progression + season history + return incentives
- [ ] [POST-ALPHA] 12.6 TSA wipe at season end (all holdings and pending trades; TSAs never carry over)

---

## Post‑Alpha / Beta — Persistent Meta Currency (Canon Only)
- [ ] [POST-ALPHA] Introduce persistent meta currency (Beta) for cosmetics/identity only; non‑tradable, non‑competitive, season‑persistent.
- [ ] [POST-ALPHA] Implement reward grant logic for persistent meta currency (non‑economic, cosmetic only).
- [ ] [POST-ALPHA] Expose persistent meta currency in UI (cosmetics, titles, badges, collections).
- [ ] [POST-ALPHA] Enforce and document: no conversion paths into Coins or Stars, direct or indirect.
- [ ] [POST-ALPHA] (Post‑Release optional) Influence/reputation metric: non‑spendable, eligibility/visibility‑only, never convertible.

## Post‑Alpha / Beta — Tradable Seasonal Assets (TSAs)

_TSAs are seasonal, player‑owned competitive assets (not currencies) introduced in Beta._

- [ ] [POST-ALPHA] Define TSA canon constraints in code (Beta‑only, seasonal competitive asset, system‑minted only, observable supply, no conversion into Coins/Stars, no minting Coins/Stars).
- [ ] [POST-ALPHA] Implement Star Sacrifice → TSA minting (Stars destroyed, immediate rank drop, irreversible).
- [ ] [POST-ALPHA] Implement player‑to‑player TSA trading (negotiated; server enforces legality, caps, and burn; logging).
- [ ] [POST-ALPHA] Add append‑only TSA logs (mint w/ stars_destroyed + source, trade w/ consideration + friction, activation) with admin visibility.
- [ ] [POST-ALPHA] Add TSA season‑end wipe behavior and snapshot/telemetry integration.
- [ ] [POST-ALPHA] Ensure TSA trading contributes to market pressure when enabled.

## Post‑Alpha / Beta — Brokered Trading (Coins ↔ Stars)

_Brokered trading is system‑priced, asymmetric, and costly. It is post‑alpha and currently disabled._

- [ ] [POST-ALPHA] Implement brokered Coins ↔ Stars trading (system-priced, with coin burn and time-based premium).
- [ ] [POST-ALPHA] Enforce eligibility gates:
  - Both players active and time-normalized
  - Both have recent coin spending activity (no pure hoarders)
  - Relative Star holdings within tightening ratio band
  - Coin liquidity within tightening band
  - Inflation exposure difference within tightening band
- [ ] [POST-ALPHA] Implement time-based tightening:
  - Eligibility gates tighten over time
  - Trade burn percentage rises
  - Maximum Stars per trade drops
  - Daily trade limits decrease
- [ ] [POST-ALPHA] Ensure trades always contribute to market pressure (never relieve it).
- [ ] [POST-ALPHA] Add brokered trade logging (burn amounts, eligibility deltas, market pressure contribution).
- [ ] [POST-ALPHA] Add admin visibility for brokered trading (premium, burn rate, eligibility tightness, trade logs).

## Post‑Beta — Rare Currencies (Canon Only)

_Rare currencies are special, limited‑drop currencies that enable pay‑to‑win features. Introduced in Beta (1–2 types), expanded in Release (3–5 types)._

- [ ] [POST-BETA] Define rare currency drop mechanics (random via normal gameplay; slightly influenced by anti-cheat).
- [ ] [POST-BETA] Implement rare currency storage (seasonal; reset at season end).
- [ ] [POST-BETA] Implement rare currency spending (stronger purchases, exclusive advantages; NEVER affects drop rates).
- [ ] [POST-BETA] Implement player‑to‑player rare currency trading (negotiated; server enforces legality and logging).
- [ ] [POST-BETA] Add append‑only rare currency logs (drops, spends, trades) with admin visibility.
- [ ] [POST-BETA] Document rare currency rarity scaling (rarer currency → stronger benefit).
- [ ] [POST-BETA] Ensure rare currencies feel "found, not farmed" (Diablo 2 rune market feel).
- [ ] [POST-BETA] Verify no conversion paths into Coins or Stars exist (direct or indirect).

## Post‑Beta — Player‑to‑Player Trading (TSAs + Rare Currencies)

_Player‑to‑player trading is negotiated and flexible. Introduced in Beta._

Valid trades:
- Coins ↔ Rare Currencies
- Rare Currencies ↔ Rare Currencies
- TSAs ↔ Coins
- TSAs ↔ Rare Currencies
- TSAs ↔ TSAs

Invalid trades:
- Stars (never tradable in any context)
- Meta currency (never tradable)
- Influence/reputation (never tradable)

- [ ] [POST-BETA] Implement player‑to‑player trading interface (negotiation, offers, confirmations).
- [ ] [POST-BETA] Enforce valid trade types and reject invalid combinations.
- [ ] [POST-BETA] Apply friction where appropriate (Coin burn, fees, caps).
- [ ] [POST-BETA] Log all player‑to‑player trades (feeds economic telemetry).
- [ ] [POST-BETA] Add admin visibility for player‑to‑player trades.
- [ ] [POST-BETA] Integrate trades with notifications (offers, confirmations, completions).

## Post‑Beta — Communication Systems (Direct Messaging + Forum)

_Communication systems support trading, social dynamics, and community engagement._

### Direct Messaging (Post-Alpha/Beta)

- [ ] [POST-ALPHA] Implement direct messaging between players.
- [ ] [POST-ALPHA] Integrate messaging with notifications (alerts for new messages).
- [ ] [POST-ALPHA] Integrate messaging with trades (offer negotiation, confirmation).
- [ ] [POST-ALPHA] Add messaging links from player profiles.
- [ ] [POST-ALPHA] Add messaging moderation tools (admin/moderator view, abuse reporting).

### Forum (Post-Beta)

- [ ] [POST-BETA] Implement game‑integrated forum.
- [ ] [POST-BETA] Add forum roles (admins → full moderation; moderators → limited moderation; players → standard posting).
- [ ] [POST-BETA] Support public trade negotiation, strategy discussion, alliances, and rivalries.
- [ ] [POST-BETA] Integrate forum with notifications (replies, mentions, trade offers).
- [ ] [POST-BETA] Integrate forum with player profiles (reputation, badges, contact links).
- [ ] [POST-BETA] Add forum moderation tools (post flags, bans, thread locks).

### Communication Philosophy

Communication systems must evoke:
- **Diablo 2 rune market**: Player-driven, social, economic
- **Competitive tension**: Rivalries, alliances, betrayals
- **Community identity**: Long-term relationships, reputation, history

## Bug Reporting & Feedback System (Alpha → Post‑Release)

_Bug reporting intake is always available from Alpha onward and persists after release._

- [x] [DONE] Implement in-game bug report intake UI (footer/help entry).
- [x] [DONE] Persist bug reports as append-only, immutable records (player_id optional, season_id, timestamp, client version if available).
- [x] [DONE] Add read-only admin visibility for bug reports (view only; no edit/delete/respond).
- [x] [DONE] Confirm Alpha has no attachments and no player feedback loop.
- [ ] [POST-ALPHA] Add admin/moderator bug triage interface.
- [ ] [POST-ALPHA] Integrate bug reports with notifications (alerts for admins/moderators).
- [ ] [POST-ALPHA] Support screenshots/logs attachment for bug reports.
- [ ] [POST-ALPHA] Track bug report status (open, in-progress, resolved, closed).
- [ ] [POST-ALPHA] Define admin/moderator response workflows (if ever; not in Alpha).

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
- [x] [DONE] 15.3 Document safe patching & schema evolution rules
- [x] [DONE] 15.4 Formalize Codex governance constraints in docs
- [x] [DONE] 15.5 Document multi-season telemetry & historical integrity guarantees

---

## Final Verification Checklist (Maintain Continuously)
- [ ] [ALPHA REQUIRED] No task depends on a later task
- [ ] [ALPHA REQUIRED] All README requirements represented (or explicitly deferred)
- [ ] [ALPHA REQUIRED] Code‑existing features verified (not just implemented)
- [ ] [ALPHA REQUIRED] Path from today → Alpha → Post‑Alpha is continuous

---

## Phase 16 — Population‑Invariant Economy Validation
- [ ] [ALPHA REQUIRED] Audit all population‑coupled inputs (coins in circulation, active coins, market pressure, affordability guardrails) and document effects at 1/5/500 players.
- [ ] [ALPHA REQUIRED] Verify Universal Basic Income (UBI) provides stable minimum income at all population levels (0.001 coin/tick).
- [ ] [ALPHA REQUIRED] Verify active circulation window (24h) produces stable emission at low population; adjust if faucet starvation occurs.
- [ ] [ALPHA REQUIRED] Add telemetry for activeCoinsInCirculation + activePlayers and confirm visibility in admin telemetry.
- [ ] [ALPHA REQUIRED] Run stress tests: solo season, small group (5‑10), large group (500+) using simulation + live tick metrics.
- [ ] [ALPHA REQUIRED] Validate faucet pacing (UBI + daily/activity/passive if enabled) against emission pool under low‑population conditions.
- [ ] [ALPHA REQUIRED] Confirm star pricing remains purchasable for a solo player over full season without trivializing scarcity.
- [ ] [ALPHA REQUIRED] Document UBI + emission + pricing interaction in coin-emission.md and coin-faucets.md.
