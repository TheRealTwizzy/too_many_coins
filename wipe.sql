BEGIN;

-- Ensure any deferrable constraints are checked at commit
SET CONSTRAINTS ALL DEFERRED;

-- Telemetry / audit logs
TRUNCATE player_telemetry RESTART IDENTITY;

-- Notifications
TRUNCATE notification_reads;
TRUNCATE notifications RESTART IDENTITY;

-- Auth/session artifacts
TRUNCATE password_resets;
TRUNCATE refresh_tokens RESTART IDENTITY;
TRUNCATE sessions;

-- IP abuse / throttling / whitelist
TRUNCATE player_ip_associations;
TRUNCATE ip_whitelist_requests;
TRUNCATE ip_whitelist;

-- Faucet / sinks / purchase logs
TRUNCATE player_faucet_claims;
TRUNCATE player_star_variants;
TRUNCATE player_boosts;
TRUNCATE star_purchase_log RESTART IDENTITY;

-- Season archives / leaderboards / economy state
TRUNCATE season_final_rankings;
TRUNCATE season_end_snapshots;
TRUNCATE season_economy;

-- Accounts and players (including bots)
TRUNCATE accounts;
TRUNCATE players;

-- Global settings (including alpha/test/playtest flags)
TRUNCATE global_settings;

COMMIT;
