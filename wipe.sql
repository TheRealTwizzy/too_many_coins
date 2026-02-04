BEGIN;

-- Ensure any deferrable constraints are checked at commit
SET CONSTRAINTS ALL DEFERRED;

-- Telemetry / audit logs
TRUNCATE player_telemetry RESTART IDENTITY;
TRUNCATE admin_audit_log RESTART IDENTITY;
TRUNCATE abuse_events RESTART IDENTITY;
TRUNCATE coin_earning_log RESTART IDENTITY;
TRUNCATE player_abuse_state;
TRUNCATE account_abuse_reputation;

-- Notifications
TRUNCATE notification_acks;
TRUNCATE notification_deletes;
TRUNCATE notification_settings;
TRUNCATE notification_reads;
TRUNCATE notifications RESTART IDENTITY;

-- Auth/session artifacts
TRUNCATE password_resets;
TRUNCATE auth_rate_limits;
TRUNCATE admin_bootstrap_tokens;
TRUNCATE admin_password_gates RESTART IDENTITY;
TRUNCATE refresh_tokens RESTART IDENTITY;
TRUNCATE sessions;

TRUNCATE player_ip_associations;

-- Faucet / sinks / purchase logs
TRUNCATE player_faucet_claims;
TRUNCATE player_star_variants;
TRUNCATE player_boosts;
TRUNCATE star_purchase_log RESTART IDENTITY;

-- Season archives / leaderboards / economy state
TRUNCATE season_calibration;
TRUNCATE season_final_rankings;
TRUNCATE season_end_snapshots;
TRUNCATE season_economy;

-- Accounts and players (including bots)
TRUNCATE accounts;
TRUNCATE players;

-- Global settings (including alpha/test/playtest flags)
TRUNCATE global_settings;

COMMIT;
