# Alpha Execution Plan (First Playable)

This plan defines the minimum goals and metrics for the first alpha test. It aligns with the first‑playable scope and server‑authoritative economy.

## Goals

- Verify inflation pacing across the Alpha season curve (14 days by default, or accelerated test window).
- Verify bulk‑buy deterrence is strong enough late‑season.
- Observe early vs late‑joiner behavior and perceived fairness.
- Identify abuse vectors around faucets, IP limits, and cooldowns.
- Validate server authority and atomicity under light load.

## Success Criteria

- Players can earn coins via daily login and active play.
- Players can buy stars (single + bulk) and prices rise over time.
- Coin scarcity is felt late‑season without total liquidity collapse.
- No client‑side trust or price manipulation is possible.

## Metrics to Track

Economy:
- Coins emitted per hour (from emission pool)
- Coins earned per hour (faucets; drip is post‑alpha and disabled in current build)
- Coins in circulation
- Stars purchased per hour
- Average star price per hour
- Market pressure value over time

Faucets:
- Daily login claims per day
- Activity claims per hour
- Claim rejection rate (cooldown, daily cap, emission exhausted)

Purchases:
- Star purchase attempts vs success rate
- Bulk purchase distribution (qty and total cost)
- Coin burn totals

Abuse/Access:
- IP‑blocked events
- Rate‑limited signup/login counts
- Account cooldown rejections

## Telemetry Events (Current Build)

- buy_star
- join_season

Alpha note: additional faucet, emission, and pressure events are planned but not yet emitted.

## Test Window

- Recommended: 7–14 days
- Consider accelerated season start via SEASON_START_UTC for testing

## Recruitment (Owner Task)

- Recruit 20–50 testers with mixed activity levels
- Collect feedback on late‑season scarcity and price clarity

## Post‑Test Review

- Review telemetry and identify top 3 economy risks
- Produce prioritized fixes before widening access
