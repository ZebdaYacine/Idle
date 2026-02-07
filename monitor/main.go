//go:build windows
// +build windows

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procGetTickCount64   = kernel32.NewProc("GetTickCount64")
)

type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

type POINT struct {
	X int32
	Y int32
}

// getIdleDuration returns how long the user has been idle (no mouse/keyboard input).
func getIdleDuration() (time.Duration, error) {
	var lii LASTINPUTINFO
	lii.CbSize = uint32(unsafe.Sizeof(lii))

	r1, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r1 == 0 {
		return 0, err
	}

	tick64, _, _ := procGetTickCount64.Call()
	now := uint64(tick64)

	last := uint64(lii.DwTime) // 32-bit tick at last input
	now32 := now & 0xFFFFFFFF  // low 32 bits of current tick

	var idleMillis uint64
	if now32 >= last {
		idleMillis = now32 - last
	} else {
		idleMillis = (0x100000000 - last) + now32
	}
	return time.Duration(idleMillis) * time.Millisecond, nil
}

// getMousePos returns the current mouse cursor position (screen coordinates).
func getMousePos() (POINT, error) {
	var p POINT
	r1, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r1 == 0 {
		return POINT{}, err
	}
	return p, nil
}

type Config struct {
	SampleEvery          time.Duration
	ActiveIfIdleLessThan time.Duration
	PrintMouseMoveEvery  time.Duration

	LogDir      string
	LogBaseName string
	FlushEvery  time.Duration

	// rqlite settings
	RqliteBaseURL string // e.g. "http://192.168.1.6:4001"
	RqliteUser    string // optional basic auth username
	RqlitePass    string // optional basic auth password

	// identity fields (kept for logs; not inserted unless your table has columns)
	HostName string
	UserName string
}

type RotatingLogger struct {
	mu      sync.Mutex
	dir     string
	base    string
	curDate string
	file    *os.File
	logger  *log.Logger
}

func NewRotatingLogger(dir, base string) (*RotatingLogger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	rl := &RotatingLogger{dir: dir, base: base}
	if err := rl.rotateIfNeeded(time.Now()); err != nil {
		return nil, err
	}
	return rl, nil
}

func (r *RotatingLogger) filenameFor(t time.Time) string {
	d := t.Format("2006-01-02")
	return filepath.Join(r.dir, fmt.Sprintf("%s-%s.log", r.base, d))
}

func (r *RotatingLogger) rotateIfNeeded(now time.Time) error {
	date := now.Format("2006-01-02")
	if date == r.curDate && r.file != nil && r.logger != nil {
		return nil
	}

	if r.file != nil {
		_ = r.file.Sync()
		_ = r.file.Close()
		r.file = nil
		r.logger = nil
	}

	path := r.filenameFor(now)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	r.curDate = date
	r.file = f
	r.logger = log.New(f, "", 0)
	return nil
}

func (r *RotatingLogger) Println(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	_ = r.rotateIfNeeded(time.Now())
	if r.logger != nil {
		r.logger.Println(line)
	}
}

func (r *RotatingLogger) Sync() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		_ = r.file.Sync()
	}
}

func (r *RotatingLogger) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		_ = r.file.Sync()
		_ = r.file.Close()
		r.file = nil
		r.logger = nil
	}
}

// statusFor returns OFF/LOW/ACTIVE/HIGH_PRODUCTION based on activity%.
func statusFor(activityPct float64, samplesInHour int) string {
	if samplesInHour == 0 || activityPct == 0 {
		return "OFF"
	}
	if activityPct < 50.0 {
		return "LOW"
	}
	if activityPct < 60.0 {
		return "ACTIVE"
	}
	return "HIGH_PRODUCTION"
}

// --- rqlite helpers (robust) ---

type rqliteExecuteResp struct {
	Results []struct {
		LastInsertID int64  `json:"last_insert_id"`
		RowsAffected int64  `json:"rows_affected"`
		Error        string `json:"error"`
	} `json:"results"`
	Error string `json:"error"`
}

// escapeSQLString escapes double quotes for SQL strings we wrap in "..."
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

// rqliteExec posts SQL statements to rqlite /db/execute and validates JSON result errors.
func rqliteExec(httpClient *http.Client, cfg Config, stmts []string) error {
	if cfg.RqliteBaseURL == "" {
		return fmt.Errorf("RqliteBaseURL is empty")
	}

	body, err := json.Marshal(stmts)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", cfg.RqliteBaseURL+"/db/execute", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if cfg.RqliteUser != "" {
		req.SetBasicAuth(cfg.RqliteUser, cfg.RqlitePass)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("rqlite execute failed: HTTP %s body=%s", resp.Status, string(respBytes))
	}

	var parsed rqliteExecuteResp
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return fmt.Errorf("rqlite execute: cannot parse JSON: %v body=%s", err, string(respBytes))
	}

	if parsed.Error != "" {
		return fmt.Errorf("rqlite execute error: %s", parsed.Error)
	}

	for i, r := range parsed.Results {
		if r.Error != "" {
			return fmt.Errorf("rqlite SQL error (stmt %d): %s", i, r.Error)
		}
	}

	return nil
}

// insertHourly inserts (or replaces) one hourly row into an already-existing table.
//
// IMPORTANT: This matches YOUR schema:
// hour_start (TEXT PK), activity_pct (REAL), idle_seconds (REAL), samples (INTEGER), status (TEXT), created_at (TEXT)
func insertHourly(httpClient *http.Client, cfg Config, hourStart time.Time, activityPct float64, idleSeconds float64, samples int, status string, createdAt time.Time) error {
	stat := escapeSQLString(status)

	stmt := fmt.Sprintf(
		`INSERT OR REPLACE INTO activity_hourly(hour_start, activity_pct, idle_seconds, samples, status, created_at)
         VALUES ("%s", %.4f, %.0f, %d, "%s", "%s");`,
		hourStart.UTC().Format("2006-01-02T15:00:00Z"),
		activityPct,
		idleSeconds,
		samples,
		stat,
		createdAt.UTC().Format(time.RFC3339),
	)

	return rqliteExec(httpClient, cfg, []string{stmt})
}

func main() {
	hn, _ := os.Hostname()
	un := os.Getenv("USERNAME")

	cfg := Config{
		SampleEvery:          1 * time.Second,
		ActiveIfIdleLessThan: 30 * time.Second,
		PrintMouseMoveEvery:  0,

		LogDir:      `C:\ProgramData\ActivityMonitor`,
		LogBaseName: "activity",
		FlushEvery:  5 * time.Second,

		// rqlite node on your LAN
		RqliteBaseURL: "http://192.168.1.6:4001",
		RqliteUser:    "",
		RqlitePass:    "",

		HostName: hn,
		UserName: un,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rot, err := NewRotatingLogger(cfg.LogDir, cfg.LogBaseName)
	if err != nil {
		fmt.Println("Cannot create rotating logger:", err)
		return
	}
	defer rot.Close()

	writeLine := func(line string) { rot.Println(line) }

	flushTicker := time.NewTicker(cfg.FlushEvery)
	defer flushTicker.Stop()

	lastMouse, err := getMousePos()
	if err != nil {
		writeLine("GetCursorPos error: " + err.Error())
		return
	}

	var (
		lastMousePrint  time.Time
		lastMouseMoveAt time.Time
	)

	httpClient := &http.Client{Timeout: 8 * time.Second}

	// Hourly counters
	hourStart := time.Now().Truncate(time.Hour)
	idleSecondsInHour := 0.0
	samplesInHour := 0

	ticker := time.NewTicker(cfg.SampleEvery)
	defer ticker.Stop()

	writeLine(fmt.Sprintf("[%s] START host=%s user=%s rqlite=%s", time.Now().Format(time.RFC3339), cfg.HostName, cfg.UserName, cfg.RqliteBaseURL))

	for {
		select {
		case <-ctx.Done():
			writeLine(fmt.Sprintf("[%s] STOP", time.Now().Format(time.RFC3339)))
			return

		case <-flushTicker.C:
			rot.Sync()

		case now := <-ticker.C:
			ts := now.Format(time.RFC3339)

			// Hour rollover: compute + INSERT once per hour
			curHour := now.Truncate(time.Hour)
			if curHour.After(hourStart) {
				activityPct := 0.0
				if samplesInHour > 0 {
					idleRatio := idleSecondsInHour / 3600.0
					if idleRatio < 0 {
						idleRatio = 0
					}
					if idleRatio > 1 {
						idleRatio = 1
					}
					activityPct = (1.0 - idleRatio) * 100.0
				}

				status := statusFor(activityPct, samplesInHour)

				if err := insertHourly(httpClient, cfg, hourStart, activityPct, idleSecondsInHour, samplesInHour, status, now); err != nil {
					writeLine(fmt.Sprintf("[%s] RQLITE insert error: %v", ts, err))
				} else {
					writeLine(fmt.Sprintf("[%s] RQLITE insert ok: hour=%s activity=%.0f%% idleSeconds=%.0f samples=%d status=%s",
						ts,
						hourStart.UTC().Format("2006-01-02T15:00:00Z"),
						activityPct,
						idleSecondsInHour,
						samplesInHour,
						status,
					))
				}

				// Reset counters for the new hour
				hourStart = curHour
				idleSecondsInHour = 0
				samplesInHour = 0
			}

			// Poll idle time and update hourly counters
			idleNow, idleErr := getIdleDuration()
			idleStr := "unknown"
			if idleErr == nil {
				idleStr = idleNow.String()
				samplesInHour++
				// NOTE: your original logic counts "idle seconds" when idle >= threshold
				// If you intended the opposite (count idle when user IS idle), keep as-is.
				if idleNow >= cfg.ActiveIfIdleLessThan {
					idleSecondsInHour += cfg.SampleEvery.Seconds()
				}
			}

			// Mouse move event logging (file only)
			p, err := getMousePos()
			if err != nil {
				writeLine(fmt.Sprintf("[%s] GetCursorPos error: %v", ts, err))
				continue
			}
			if p.X == lastMouse.X && p.Y == lastMouse.Y {
				continue
			}

			prevMoveStr := "first_move"
			if !lastMouseMoveAt.IsZero() {
				prevMoveStr = lastMouseMoveAt.UTC().Format(time.RFC3339)
			}

			if cfg.PrintMouseMoveEvery == 0 || lastMousePrint.IsZero() || now.Sub(lastMousePrint) >= cfg.PrintMouseMoveEvery {
				writeLine(fmt.Sprintf("[%s] EVENT=MOUSE_MOVE pos=(%d,%d) prevMouseMoveAt=%s idleNow=%s",
					ts, p.X, p.Y, prevMoveStr, idleStr))
				lastMousePrint = now
			}

			lastMouse = p
			lastMouseMoveAt = now

			_ = idleErr
		}
	}
}
