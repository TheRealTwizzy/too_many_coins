# Communication & Bug Reporting

This document defines player communication boundaries and the bug reporting intake system.

---

## Bug Reporting Intake (Player-Facing)

Bug reporting is always available from Alpha onward and is never removed post-release.

Player-facing access:

- Entry point from the main UI (footer or help menu).
- Fields: short title, free-text description, optional category (UI, Economy, Performance, Other).
- No attachments in Alpha.
- No direct developer responses in Alpha.
- Submitting a report does not affect gameplay, economy, rank, or trust.

---

## Backend Handling (Conceptual)

Bug reports are append-only records. Each report is immutable once submitted.

Each report includes:

- player_id (if logged in)
- season_id
- timestamp
- client version (if available)

---

## Admin Visibility (Read-Only, Alpha)

Admins can view bug reports in the admin console. Admin access is observational only:

- No edit
- No delete
- No responses to players

---

## Governance Rules

Bug reporting is not a communication channel and does not replace forums or messaging.

- Bug reporting must not be rate-limited in a way that blocks normal use.
- Abuse of bug reporting is handled via existing anti-abuse throttles, not special punishment.
