# Too Many Coins!

Too Many Coins! is an online-only massively multiplayer website game built around inflation, scarcity, and shared economic pressure. It is designed for everyone and supports many thousands of concurrent players in each season. The game is simple to understand but strategically deep over time.

---

## Overview

Players earn Coins and spend them to buy Stars. Stars determine leaderboard rank directly for a season; TSAs can influence outcomes indirectly through their utility. As more Coins enter the system and as time passes, Stars become increasingly expensive. Coin supply shrinks as the season progresses, creating scarcity and tension, especially near the end. Coin shortage is possible but rare; the system stays liquid enough for daily action.

The game runs in fixed-length seasons and resets regularly, while preserving long-term player progression through cosmetics, titles, badges, and history.

---

## Alpha Scope (Current Build)

Alpha is focused on the first playable economy loop:

- Single active season only (no season lobby)
- Trading is disabled (post‑alpha)
- TSAs are disabled (post‑alpha)
- Passive drip is disabled (post‑alpha)
- Daily tasks and comeback rewards are disabled (post‑alpha)
- Admin economy controls are read‑only
- Market pressure is derived from star purchases only
- Anti‑abuse protections are minimal but real (rate limiting + cooldowns)

---

## Core Design Principles

The game must be simple, transparent, and fair  
All economy logic must be enforced server-side  
Bulk buying must be technically allowed but economically discouraged  
Late-season scarcity must feel intense but still rewarding  
Economic pressure must curve, not cliff  
There is always a rational action and a cost to inaction  
There is never a safe move  
The system must resist coordinated manipulation and bad actors  
Players must have reasons to stay active until the end of a season and return for future seasons  

---

## Seasons

Season length is phase‑bound and server‑defined:

- Alpha: 14 days by default. Extension up to 21 days is allowed **only** when explicitly configured for telemetry gaps. Single active season only.
- Beta: 28 days. Total seasons: 2–3. Seasons may overlap and are staggered.
- Release: 28 days. Concurrent seasons: up to 4, staggered.

Players may join any active season at any time.  
Each season has its own independent economy.  
Coins and Stars reset at the end of each season.  
Persistent rewards carry over between seasons (post‑alpha).

Season day index and total days are server-authoritative; clients must render the provided values and never hardcode 28-day assumptions.

At season end (Alpha):

- Economy actions are frozen (no earning, no purchases).
- Clients display a single terminal state: **Ended** (no “Ending” state exposed).
- Live economy rates (emission, inflation/pressure cadence) are hidden; UI shows a frozen/final snapshot marker instead.
- Ended seasons expose final snapshot fields only (final star price, final coins in circulation, ended at).

---

## Currencies

Alpha (no change): Coins and Stars are the only currencies. No other currencies exist in Alpha.

Seasonal currencies (Alpha/Beta/Release):

Coins:

Seasonal  
Inflation‑controlled  
Faucet‑based  
Reset every season

Stars:

Seasonal  
Competitive  
Leaderboard‑defining  
Reset every season  
Not used for cosmetics or meta progression

Post‑Alpha persistent meta currency (Beta):

Introduced in Beta  
Persists across seasons  
Cosmetic / identity use only  
Cannot be traded  
Cannot convert into Coins or Stars  
Cannot affect competitive power

Optional influence / reputation metric (Post‑Release):

Non‑spendable  
Eligibility / visibility modifier only  
Never convertible  
Not required for Beta

Competitive assets (Post‑Alpha / Beta):

Tradable Seasonal Assets (TSAs)

Seasonal, player‑owned competitive assets (not currencies)
Freely tradable player‑to‑player
System‑minted only, supply observable
Utility‑bearing and strategically risky

Hard prohibition:

> No currency may ever convert into Coins or Stars, directly or indirectly.

### Tradable Seasonal Assets (TSAs) — Post‑Alpha / Beta‑Only

TSAs are seasonal, player‑owned competitive assets (not currencies) introduced in Beta.

TSA rules:

- Beta‑only; no TSAs exist in Alpha.
- TSAs are competitive assets, not currencies.
- TSAs are system‑minted only; supply is observable and auditable.
- TSAs are freely tradable player‑to‑player; trades are player‑negotiated.
- The system enforces legality, caps, and logging; it does not set prices.
- Trades may include friction (Coin burn, Star burn, caps).
- TSAs never mint Coins or Stars and never convert into Coins or Stars, directly or indirectly.
- Stars sacrificed for TSAs are permanently destroyed; leaderboard rank drops immediately.
- TSAs reset at season end; no carryover between seasons.
- Trading remains disabled in Alpha.

TSA acquisition paths:

1) Player‑to‑player trade
	- Open negotiation
	- System enforces legality and logs the trade

2) Star Sacrifice (System Exchange)
	- Player permanently destroys Stars
	- Player receives a TSA
	- Leaderboard rank drops immediately
	- TSAs cannot be converted back into Stars

TSA philosophy & risks:

- Irreversible and season‑bound; mistakes are permanent.
- Scarce, utility‑bearing, and strategically dangerous.
- Designed to introduce regret, risk, and high‑stakes tradeoffs.
- Competitive impact is indirect: TSAs change outcomes via utility, not via Stars.

Hard TSA invariants:

- TSAs cannot mint Coins.
- TSAs cannot mint Stars.
- TSAs cannot be converted into Coins or Stars.
- Stars sacrificed for TSAs are permanently destroyed.
- TSA supply is observable and auditable (no hidden supply).

---

## Core Gameplay Loop

Players earn Coins through daily login and active play faucets.  
Passive drip is post‑alpha and disabled in the current build.  
Daily tasks and comeback rewards are post‑alpha and disabled in the current build.  
Alpha safeguard: on login, the server may top up very low balances to keep the game playable within minutes (draws from the emission pool, short cooldown).  
Players spend Coins to buy Stars.  
Players may optionally trade Coins for existing Stars under tight, time-worsening constraints (post‑alpha).  
Post‑alpha: players may sacrifice Stars to obtain TSAs or trade TSAs player‑to‑player.  
Star prices increase over time and with demand.  
Coin supply decreases over time.  
Inflation pressure increases monotonically; delay is punished and mistakes are permanent.  
Late-season decisions become harder and more consequential.

---

## Trading (Conditional, Brokered — Coins ↔ Stars)

_Trading is post‑alpha and currently disabled. The following describes the planned system._

Trading is optional, costly, asymmetric, and increasingly restrictive as the season progresses.

Trading rules:

Brokered trading refers to Coins ↔ Stars only; TSA trading is separate and player‑negotiated.

Trades are Coins-for-Stars only (no Coin-for-Coin, no Star-for-Star).  
Trades are brokered by the system; players do not set prices.  
Every trade burns Coins as overhead. Burned Coins never re-enter the economy.  
Trades never create Coins or Stars and never bypass scarcity.  
Trades are asymmetric: the buyer pays more than the seller receives due to burn and fees.  
Trades are priced at or above the current system star price, plus a time-based premium.  
Trades always contribute to market pressure, never relieve it.  

TSA trading (post‑alpha, Beta‑only):

- Player‑to‑player negotiated; the system does not set prices.
- The system enforces legality, caps, and logging; friction may apply.
- Never creates Coins or Stars.
- Never converts into Coins or Stars.
- Always contributes to market pressure when enabled.

Eligibility gates (must pass all):

Both players must be currently active and time-normalized participants.  
Both must have recent coin spending activity (no pure hoarders).  
Relative Star holdings must be within a tightening ratio band.  
Coin liquidity must be within a tightening band (not too low, not too high).  
Inflation exposure difference must be within a tightening band.  

Some players will not qualify to trade. Some pairs will never qualify.

As the season progresses:

Trade eligibility gates tighten.  
Trade burn percentage rises.  
Maximum Stars per trade drops.  
Daily trade limits decrease.  

Late-season trading is expensive, dangerous, and narrow, but still rational in specific cases.

Typical rational cases: a seller needs liquidity to keep playing, or a buyer pays a premium to reach a tier when time is short.

---

## Trading as Pressure, Not Relief

Trading does not save players from inflation. It adds pressure:

Trades burn Coins and reduce total liquidity.  
Trades are priced with a premium and never undercut the system price.  
Trades increase market pressure and can make future Stars more expensive.  
Eligibility tightens over time and can deny trades entirely.  

Trading is a costly tool for repositioning risk, not a catch-up system.

---

## Star Pricing

Star prices must be dynamic and depend on multiple factors:

Time progression across the phase‑bound season length (14 days in Alpha; 28 days in Beta/Release)  
Purchase quantity, with non-linear scaling for bulk purchases  
Market pressure based on recent star-buying activity  
A late-season spike that sharply increases prices in the final week  

Star prices must remain affordable relative to per-player coin emission. Prices should track average coins per player so most active players can still buy stars throughout the season.

Bulk purchases must scale so aggressively that they become almost infeasible late in the season. Bulk buying should only be viable early for players attempting to gain an early lead.

Market pressure must be smoothed using rolling averages and rate limits so prices cannot spike instantly. Coordinated manipulation on day one must be ineffective.

---

## Coin Supply and Inflation

Coins are introduced through controlled, server-managed inflation.  
Coins must never be created directly by player-triggered actions.

Coin emission rules:

Coins are emitted continuously by the server  
A global coin budget exists for each day and decreases over the season  
Players earn Coins by drawing from this pool via limited faucets  
Individual player earning caps decrease over time  
Late-season coin supply is significantly scarcer  

Coin emission must be time-sliced so a full day’s supply cannot be drained instantly. If coin consumption is too fast, earning rates must be throttled smoothly rather than stopped abruptly.

Trade burn is modeled and balanced against minting to maintain liquidity. Coin shortage is possible but rare; the system must remain liquid enough for meaningful daily action.

---

## Late-Season Design

Late-season play must feel tense but worthwhile.

Late in the season:
Star prices are much higher  
Coin supply is much lower  

Late-season incentives must not inject large amounts of Coins. Rewards should be non-economic and persistent, such as:

Badges  
Titles  
Cosmetics  
Achievements  
Participation recognition  
Community-wide progress rewards  

Late-season play also includes:

Small, high-impact star purchases  
Tight, costly trading opportunities  
Late-season challenges that reward persistence without adding Coins

---

## Anti-Manipulation and Abuse Prevention

One active player per IP address per season is the default baseline.

If multiple accounts originate from the same IP:
They are not hard-blocked; they are throttled through economic dampening, cooldowns, and trust-based enforcement.
No whitelist requests or manual approvals are used in alpha.

Players earn Coins faster while actively using the site. Passive drip is post‑alpha and disabled in the current build.

Admin tools (alpha):
Read‑only economy monitoring and telemetry. No direct coin/star edits.

Additional protections:
Rate‑limited account creation  
Cooldown before new accounts can join a season  
CAPTCHA and verification (post‑alpha)  
Detection of suspicious clustering or coordinated behavior (post‑alpha)  
Automatic throttles for suspicious market activity (post‑alpha)  

---

## Retention Between Seasons

Players must be motivated to return season after season.

Persistent progression includes:
Account level  
Cosmetic collections  
Badges and titles  
Season history  

Each season may include a simple modifier that changes presentation or rewards without altering core economic rules.

---

## Changelog (Alpha)

- [Alpha] Removed whitelisting system
- Simplified access control and admin overhead
- Relies fully on server-side anti-abuse and economic pressure
- No player-facing permission gating remains
- [Alpha] Expanded role-based notification system
- Added player, moderator, and admin notification tiers
- Introduced priority alerts for admin-critical events
- Improved observability without affecting economy behavior
- [Alpha] Introduced passive anti-cheat behavior monitoring
- Server now observes long-term abuse patterns and applies quiet throttles
- No player-facing penalties or rewards
- Improves economy integrity and resistance to manipulation

## Website Pages

Landing page explaining the game quickly  
Authentication page for signup and login  
Main season dashboard where gameplay occurs  
Bulk purchase interface with transparent cost scaling  
Leaderboard page  
Internal admin console for moderation and economy monitoring  

Post‑alpha pages:
Season lobby showing all active seasons  
Player profile and collection page  
Settings and accessibility page  
Trading desk

---

## Bot Runner (Testing)

The bot runner uses the same public HTTP APIs as players and is intended for load/behavior testing. See [README/bot-runner.md](README/bot-runner.md).

---

## Notifications and Password Reset

Notifications are delivered in-app and can be managed from the admin console. Password resets use:

- POST /auth/request-reset
- POST /auth/reset-password

Email delivery requires SMTP configuration via environment variables (SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM).

---

## Roles and Admin Workflow

See the admin governance sections below. Admin creation is not a gameplay feature during Alpha (bootstrap is server-only).

## Admin Bootstrap (Alpha)

- On first startup after a fresh DB reset, the server auto‑creates exactly one admin account (username `alpha-admin`).
- The admin account is created locked (`must_change_password = true`).
- The owner sets `OWNER_CLAIM_SECRET` once as a Fly secret (not a login password).
- The owner visits `/admin/initialize` and enters the rotating owner claim code to set the first admin password.
- All admin endpoints are blocked until the password is changed.
- Bootstrap is sealed in the database and cannot repeat unless the DB is wiped.
- If bootstrap is sealed but no admin exists, the server refuses to start (safety invariant).

Notes:

- The claim code rotates automatically (short time windows) and is derived from `OWNER_CLAIM_SECRET`.
- The claim code is never stored and is not logged.
- After the claim, future admin password changes use the standard reset flow.

## Admin Management During Alpha

- Additional admins are assigned via direct database updates only.
- This is an operational (ops) action, not a gameplay feature.
- Changes should be deliberate and auditable.
- No client or API-based admin escalation exists during Alpha.

Note: `/admin/role` is disabled in Alpha; role changes are DB‑only.

### Standardized DB Procedures (psql-safe)

Promote an account to admin:

UPDATE accounts
SET role = 'admin'
WHERE account_id = 'ACCOUNT_ID_HERE';

Demote an admin:

UPDATE accounts
SET role = 'player'
WHERE account_id = 'ACCOUNT_ID_HERE';

Verify current admins:

SELECT account_id, username, role
FROM accounts
WHERE role IN ('admin', 'frozen:admin')
ORDER BY username;

---

## Deployment (Fly.io)

The server auto-creates schema on startup. For Fly.io:

- Build: Dockerfile (multi-stage)
- Start: ./app
- Health check: /health

Set DATABASE_URL and any required secrets (SMTP_* if enabling email). Ensure PHASE=alpha (or APP_ENV=alpha fallback) on Fly. For manual migrations, use schema.sql.

Alpha-only season extension (telemetry gaps):

- ALPHA_SEASON_EXTENSION_DAYS (max 21)
- ALPHA_SEASON_EXTENSION_REASON (required when extension is set)

Health checks verify:

- Database connectivity

---

## Alpha Reset (ALPHA-ONLY)

Use the guarded reset script to wipe the database during Alpha testing:

1) Set PHASE=alpha (or APP_ENV=alpha fallback)
2) Set ALPHA_RESET_CONFIRM=I_UNDERSTAND
3) Run scripts/alpha-reset.sh

The script drops the public schema, re-applies schema.sql, and is intentionally gated to prevent accidental use in production.

---

## Monitoring (Minimum Viable)

- /health for uptime checks
- /admin/telemetry for economy/event visibility
- Admin economy dashboard for read-only snapshots

---

## Technical Requirements

The game must scale to thousands of concurrent players per season.  
All economy calculations must be server-side only.  
Purchases must be atomic and race-condition safe.  
Real-time updates should use WebSockets or server-sent events.  
The client must never be trusted for economic logic.  
The game is online-only and web-based.

---

## Design Goal

A new player should understand the game immediately.  
An experienced player should find strategy and tension.  
Late-season play should remain meaningful.  
Players should return for multiple seasons.

---

## Mid-Season and Late-Season Play

Mid-season and late-season are designed to be risky, costly, and narrow, but never pointless.

Late joiners are disadvantaged but not invalidated.

You can still:

Earn Coins through limited faucets  
Buy Stars in small, high-impact quantities  
Trade under strict eligibility to reposition risk  
Chase tiers, badges, and late-season challenges  

You cannot:

Catch up safely  
Erase mistakes  
Avoid inflation pressure  

---

## Why You Can Still Play (Even If You Can’t Win)

You may be mathematically unable to reach first place, but you always have meaningful decisions:

Spend now vs. wait and risk higher prices  
Buy a small number of Stars vs. save for a later spike  
Sell Stars to regain liquidity vs. hold position  
Use a costly trade to reach a tier vs. accept rank decay  

There is always something at stake, and inaction always has a cost.

---

## Design Guarantee (Revised)

There is always a rational action.  
There is always something at stake.  
There is always a cost to inaction.  
There is never a safe move.  
Hope may exist; comfort must not.

---
