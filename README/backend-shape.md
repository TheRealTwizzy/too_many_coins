The backend will start as a single authoritative API service.
This service handles authentication, seasons, economy logic, purchases, and abuse prevention.

A single relational database is used for persistent state, transactions, and audit logs.

Real-time updates are delivered via a simple broadcast channel such as Server-Sent Events or WebSockets, used only for price and season state changes.

Background jobs handle coin emission, daily resets, whitelist waves, and abuse detection.

The system intentionally avoids microservices, sharding, or complex queues until scale demands them.