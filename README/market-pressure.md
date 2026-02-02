Market pressure represents recent demand for stars and influences star prices.

Market pressure is calculated using rolling averages over time and is rate-limited.

Market pressure inputs:

Total stars purchased in the last 24 hours

Total stars purchased in the last 7 days

Market pressure calculation:

Pressure increases when short-term demand exceeds long-term average demand.

Pressure decreases gradually when demand slows.

Market pressure smoothing rules:

Market pressure changes are capped per server tick.

Market pressure may not increase or decrease by more than a small percentage per hour.

Sudden spikes in demand are absorbed over time rather than applied instantly.

Market pressure is derived server-side and stored as a season-level value.
Clients only receive the current pressure value and never compute it.

Market pressure is applied multiplicatively to star prices.

Market pressure must be resistant to day-one coordinated activity and bot-driven bursts.