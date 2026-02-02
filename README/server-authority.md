The server is the sole authority over the game economy.
The client never calculates prices, limits, caps, or outcomes.

The server controls:

Current star price

Bulk purchase cost calculations

Coin emission and throttling

Player coin balances

Player star balances

Daily earning caps

Global coin budget

Market pressure values

Season timing and transitions

IP-based season access rules

Whitelist approval state



The client is responsible only for:

Displaying server-provided values

Sending player intents (earn coins, buy stars)

Rendering UI and feedback

All purchases must be validated, priced, and finalized server-side atomically.