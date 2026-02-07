package main

import "github.com/rqlite/gorqlite"

type ActivityRepo struct {
	conn *gorqlite.Connection
}

func NewActivityRepo(conn *gorqlite.Connection) *ActivityRepo {
	return &ActivityRepo{conn: conn}
}

// GetBetween returns rows in [startRFC3339, endRFC3339)
func (r *ActivityRepo) GetBetween(startRFC3339, endRFC3339 string) ([]ActivityRow, error) {
	qr, err := r.conn.QueryOneParameterized(gorqlite.ParameterizedStatement{
		Query: `SELECT hour_start, activity_pct, idle_seconds, samples, status, created_at
		        FROM activity_hourly
		        WHERE hour_start >= ? AND hour_start < ?
		        ORDER BY hour_start;`,
		Arguments: []interface{}{startRFC3339, endRFC3339},
	})
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	rows := make([]ActivityRow, 0, 16)
	for qr.Next() {
		var row ActivityRow
		if err := qr.Scan(&row.HourStart, &row.ActivityPct, &row.IdleSeconds, &row.Samples, &row.Status, &row.CreatedAt); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
