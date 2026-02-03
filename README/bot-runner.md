# Bot Runner

The bot runner is a separate process that uses the same HTTP APIs as real players. It authenticates via access + refresh tokens, and only acts when `BOTS_ENABLED=true`.

## Environment Variables

Required:
- `API_BASE_URL` — Public backend URL (e.g., https://your-app.fly.dev)
- `BOTS_ENABLED` — `true`/`false`
- `BOT_LIST` — JSON string of bot credentials and strategies **or** `BOT_LIST_PATH`

Optional:
- `BOT_LIST_PATH` — Path to a JSON file with bot configs
- `BOT_RATE_LIMIT_MIN_MS` — Minimum jitter between bots (default 3000)
- `BOT_RATE_LIMIT_MAX_MS` — Maximum jitter between bots (default 12000)
- `BOT_ACTION_PROBABILITY` — Chance to act on each bot (default 1.0)
- `BOT_MAX_ACTIONS_PER_RUN` — Max actions per run (default 1)

## Bot Config Format

```json
[
  {
    "username": "bot_alpha_01",
    "password": "LONG_RANDOM",
    "strategy": "threshold_buyer",
    "threshold": 500,
    "maxStarsPerDay": 50
  }
]
```

## Strategies

- `threshold_buyer`: buy 1 star when `currentStarPrice <= threshold` and coins >= price
- `cautious_buyer`: buy 1 star when `currentStarPrice <= coins * 0.5`
- `late_fomo`: threshold grows as the season progresses

## Local Run

```
BOTS_ENABLED=true \
API_BASE_URL=http://localhost:8080 \
BOT_LIST='[{"username":"bot_alpha_01","password":"...","strategy":"threshold_buyer","threshold":300}]' \
 go run ./cmd/bot-runner
```

## GitHub Actions

Add the following secrets:
- `API_BASE_URL`
- `BOTS_ENABLED`
- `BOT_LIST`

The workflow is defined in `.github/workflows/bot_runner.yml` and runs every 5 minutes.

## Tagging Bots

Mark bot accounts in the database (alpha).

Post‑alpha admin UI may provide Player Controls for tagging:
- Set **Bot status** to "Bot"
- Set **Bot profile** to the strategy label

Bots will appear in the leaderboard with a BOT badge.
