Game: Too Many Coins! (MMO web)

Seasons (phase‑bound, server‑defined):

- Alpha: 14 days by default; extension up to 21 days only with explicit telemetry‑gap configuration. Single active season only.
- Beta: 28 days, total seasons 2–3, staggered/overlapping.
- Release: 28 days, concurrent seasons up to 4, staggered.

Join rules: players can join any active season anytime

Core loop: earn coins (faucets) -> buy stars (system store) -> optional brokered trading -> leaderboard rank

Trading: optional, costly, asymmetric, brokered Coins-for-Stars only; system-priced with premium and coin burn; eligibility gates tighten over time

Bulk buy allowed but price scales so hard it’s almost infeasible late-season

Inflation model: server-emitted coin pool, time-sliced, global daily budget decreases over season; trade burn is modeled and balanced to preserve liquidity

Inflation pressure is monotonically increasing; delay is punished and mistakes are permanent

Late-season: star prices higher, coin supply lower, trading narrower and more expensive

Affordability: star prices stay aligned with per-player coin emission so most active players can buy stars throughout the season

Anti-abuse: 1 active account per IP per season by default; no whitelist, throttles + trust-based enforcement only

Rewards: cosmetics/titles/badges persist between seasons; coins/stars reset per season

Post‑alpha currency model: introduce a persistent meta currency (Beta) for cosmetic/identity use only; it cannot be traded, cannot convert into Coins or Stars, and cannot affect competitive power. An optional influence/reputation metric may exist post‑release; it is non‑spendable, eligibility/visibility‑only, and never convertible.

Hard prohibition: no currency may ever convert into Coins or Stars, directly or indirectly.

Real-time: prices and season stats update via SSE or WebSockets

Server-authoritative economy: all pricing, caps, purchases enforced server-side