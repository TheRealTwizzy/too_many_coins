The system must actively prevent and mitigate abuse and coordinated manipulation.

---

## Anti-Cheat Philosophy — Soft Enforcement

Anti-cheat is **gradual, invisible, and corrective**, not punitive.

### What Anti-Cheat NEVER Does

Anti-cheat **NEVER**:

- Bans automatically
- Suspends accounts automatically
- Zeroes a wallet
- Hard-blocks players
- Exposes enforcement actions publicly

### What Anti-Cheat DOES

Anti-cheat acts by **adjusting coin faucet flow**:

- Gradually reduces earning rates for suspicious behavior
- Increases star prices for suspicious accounts
- Adds cooldowns and jitter to sensitive actions
- Throttles activity without blocking it

### Enforcement Scaling

**Effects should be**:

- **Gradual**: Small adjustments at first, increasing over time
- **Mostly invisible**: Players feel resistance, not punishment
- **Severe only in extreme abuse**: Heavy throttles reserved for confirmed bad actors

**Behavior → consequence must scale smoothly**:

- Minor suspicious activity → minor throttles
- Moderate abuse patterns → noticeable resistance
- Extreme abuse → heavy economic dampening

### Admin Involvement

Admins may ban ONLY:

- **After anti-cheat recommendation** (system flags extreme cases)
- **In extreme cases** (confirmed, egregious abuse)

Admins **must not**:

- Micromanage economy
- Manually adjust individual player balances
- Override anti-cheat without justification

The goal: **Make abuse economically ineffective, not publicly punishing.**

---

## Access controls:

Only one active player per IP address per season is the default baseline.

Additional accounts from the same IP are not hard-blocked; they are throttled through economic dampening, cooldowns, and trust-based enforcement.

No whitelist or allowlist is used in alpha.

Account protections:

Account creation is rate-limited.

CAPTCHA and verification are post‑alpha.

Account age is a soft signal only. Hard gating first-session play is forbidden.
New accounts may face softer throttles (cooldown multipliers, reward dampening, bulk limits), but never hard blocks.

Throttles:

Per-player star purchase rate limits exist.

Per-IP star purchase limits exist, especially early in the season.

Coin earning and star buying may be dynamically throttled for suspicious activity.
Brokered trading eligibility may be tightened or suspended for suspicious activity.

Detection:

The system monitors for clustering patterns such as many new accounts from related IP ranges acting similarly.

Suspicious activity generates abuse events.

Abuse events may trigger automatic temporary throttles.

Trade-specific detection:

Repeated reciprocal trades between the same accounts

Trading patterns that concentrate Stars across related IP ranges

Unusual trade volume spikes relative to participation

(Trade-specific detection is post‑alpha while trading is disabled.)

Enforcement:

Throttles are gradual and reversible.

The goal is to make abuse economically ineffective, not to punish publicly.

All abuse decisions and throttles are enforced server-side.

---

## Alpha Audit — Post‑Whitelist Removal

Implemented (confirmed in code):

- Whitelisting removed; alpha relies on throttles only.
- Auth rate limits for signup/login (IP‑based windows).
- Account age used as a soft throttling signal (cooldown/reward/bulk multipliers), never a hard gate.
- IP association tracking and dampening (delay + reward/price multipliers when multiple accounts share an IP).
- Abuse scoring with throttles (earn multiplier, price multiplier, bulk max, cooldown jitter) driven by detected signals.
- Abuse signals include purchase bursts, regular purchase cadence, activity cadence, tick‑reaction patterns, and IP clustering.
- Abuse events are logged and emit moderator/admin notifications.
- Bot star‑purchase rate limit enforced via minimum interval.

Gaps / Alpha‑known limitations:

- No explicit hard per‑IP star purchase limit beyond IP dampening and abuse scoring.
- No explicit per‑player star purchase rate limit beyond abuse scoring and bot interval limits.
- CAPTCHA and verification remain post‑alpha.
- Trade‑specific abuse detection is inactive while trading is disabled.