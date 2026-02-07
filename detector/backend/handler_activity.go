package main

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

type ActivityHandler struct {
	repo *ActivityRepo
}

func NewActivityHandler(repo *ActivityRepo) *ActivityHandler {
	return &ActivityHandler{repo: repo}
}

func parseHHMM(s string) (h, m int, ok bool) {
	if len(s) != 5 || s[2] != ':' {
		return 0, 0, false
	}
	h = int((s[0]-'0')*10 + (s[1] - '0'))
	m = int((s[3]-'0')*10 + (s[4] - '0'))
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// GET /activity/today?start=07:00&end=16:00&tz=UTC&date=2026-02-07
func (h *ActivityHandler) GetToday(c *fiber.Ctx) error {
	// timezone
	tz := c.Query("tz", "UTC")
	loc := time.UTC
	if tz == "Local" {
		loc = time.Local
	}

	// date
	dateStr := c.Query("date", "")
	var day time.Time
	if dateStr == "" {
		day = time.Now().In(loc)
	} else {
		parsed, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid date (use YYYY-MM-DD)")
		}
		day = parsed
	}

	// start/end
	startStr := c.Query("start", "07:00")
	endStr := c.Query("end", "16:00")

	sh, sm, ok := parseHHMM(startStr)
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "invalid start (use HH:MM)")
	}
	eh, em, ok := parseHHMM(endStr)
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "invalid end (use HH:MM)")
	}

	start := time.Date(day.Year(), day.Month(), day.Day(), sh, sm, 0, 0, loc).Format(time.RFC3339)
	end := time.Date(day.Year(), day.Month(), day.Day(), eh, em, 0, 0, loc).Format(time.RFC3339)

	rows, err := h.repo.GetBetween(start, end)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}

	return c.JSON(fiber.Map{
		"start": start,
		"end":   end,
		"count": len(rows),
		"rows":  rows,
	})
}
