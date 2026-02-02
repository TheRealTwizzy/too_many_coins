The database stores only authoritative facts required to reconstruct the game state.

Persistent entities include:

Players:

player_id

account_id

created_at

last_login_at

trust_status (normal, throttled, flagged)

whitelist_group_id (nullable)

Seasons:

season_id

start_time

end_time

current_day

status (active, ended)

PlayerSeasonState:

player_id

season_id

coin_balance

star_balance

daily_earn_total

last_earn_reset_at

last_action_at

EconomyState (per season):

season_id

current_base_price

market_pressure

global_coin_budget_remaining

last_emission_tick_at

Purchases (append-only log):

purchase_id

player_id

season_id

star_quantity

total_coin_cost

price_snapshot

quantity_multiplier_snapshot

market_pressure_snapshot

created_at

CoinEarnings (append-only log):

earning_id

player_id

season_id

source_type (login, task, activity, comeback)

amount

created_at

WhitelistRequests:

request_id

ip_hash

requested_slots

status (pending, approved, denied)

reviewed_at

AbuseEvents:

event_id

player_id

season_id

event_type

severity

created_at

Derived values such as star prices, caps, throttles, and rankings are computed server-side and are not trusted if provided by clients.

All coin and star balance changes must occur inside database transactions.