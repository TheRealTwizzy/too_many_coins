The backend will start as a single authoritative API service.
This service handles authentication, seasons, economy logic, purchases, brokered trading, and abuse prevention.

A single relational database is used for persistent state, transactions, and audit logs.

Real-time updates are delivered via a simple broadcast channel such as Server-Sent Events or WebSockets, used only for price and season state changes.

Background jobs handle coin emission, daily resets, and abuse detection.

The system intentionally avoids microservices, sharding, or complex queues until scale demands them.

---

## Safe Patching & Schema Evolution

Schema changes are additive only during active development.

Allowed:

- New tables
- New columns (nullable or defaulted)
- New indexes

Forbidden unless explicitly authorized:

- Dropping columns or tables
- Renaming columns
- Reinterpreting existing data
- Retroactive recomputation

Old code must be able to run against new schema.  
New code may assume new schema exists.

---

## Admin Bootstrap vs Schema Responsibilities

Schema application occurs before runtime. Admin bootstrap:

- Does NOT create tables
- Does NOT migrate schema
- Does NOT alter schema

Bootstrap assumes schema correctness and only enforces safety invariants.

Bootstrap must never depend on schema evolution logic.