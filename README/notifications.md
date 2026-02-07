# Notifications (Alpha Scope)

Notifications are informational, server-generated updates for players and admins. They do not control gameplay and are never authoritative.

---

## Notification Types (Alpha)

Player-facing notifications include:

- Economy events (price changes, season milestones).
- System notices (season ended, feature disabled notices).
- Auth-related notices for password reset flow.

Admin/system alerts (admin-only):

- Priority alerts for admin-critical events.
- Abuse events that emit moderator/admin notifications.

---

## What Notifications Are Not (Alpha)

Notifications are not:

- Direct messaging
- Chat
- A forum substitute
- A moderation channel
- Guaranteed real-time delivery (best-effort only)

---

## Delivery Semantics

- Notifications are generated server-side only.
- Delivery channels include:
  - In-app notification feed
  - SSE-triggered UI updates
- Missed notifications may be visible on next login.
- No push/email requirement in Alpha; email is optional infrastructure.

---

## Admin Visibility (Read-Only, Alpha)

- Admins can view notification emission events (read-only).
- Admins cannot send custom notifications.
- Admins cannot target individual players.

---

## Failure Transparency

- If notifications fail to deliver, gameplay must not break.
- Notifications are informational only and never authoritative.
