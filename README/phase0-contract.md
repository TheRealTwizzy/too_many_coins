# Phase 0 Contract — Infrastructure Alignment

This phase prioritizes infrastructure correctness over gameplay completeness.

## Must Be Real
- Account creation
- Login via HTTP-only cookie sessions
- Session persistence across server restarts
- Player identity + persistent state
- Admin bootstrap safety

## Allowed to Be Stubbed or Disabled
- Economy math
- Seasons (single season only)
- Faucets
- Star pricing
- Leaderboard ordering
- SSE streaming

## Forbidden
- Client-side balance or price calculations
- UI placeholder values for coins/stars
- Implicit defaults when server data is missing
- Multiple sources of truth

## Exit Criteria
- A user can sign up, log in, restart the server, and see the same stored state.
