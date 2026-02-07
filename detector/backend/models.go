package main

type ActivityRow struct {
	HourStart   string  `json:"hour_start"`
	ActivityPct float64 `json:"activity_pct"`
	IdleSeconds float64 `json:"idle_seconds"`
	Samples     int64   `json:"samples"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
}
