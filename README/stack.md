Backend language: Go
Backend framework: net/http
Database: PostgreSQL
Hosting target: Fly.io
Realtime method: Server-Sent Events

---

## Codex Governance Rules (Non-Negotiable)

Every Codex change must:

- Declare STEP INTENT
- List FILES AFFECTED explicitly
- Pass STARTUP SAFETY CHECK
- End with STOP & COMMIT POINT

Additional constraints:

- One logical unit per prompt
- No opportunistic edits
- No cross-system changes
- No "while here" work

Codex is an execution tool, not a design authority.

---

## Multi-Season Telemetry & Historical Integrity

- All telemetry is season-scoped
- Past seasons are immutable
- Admin control changes are first-class telemetry
- Telemetry from past seasons may inform future seasons
- Past data is never rewritten or reinterpreted

The system evolves forward by learning, never by editing history.