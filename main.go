//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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

// --- Win API helpers ---

func getIdleDuration() (time.Duration, error) {
	var lii LASTINPUTINFO
	lii.CbSize = uint32(unsafe.Sizeof(lii))

	r1, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r1 == 0 {
		return 0, err
	}

	tick64, _, _ := procGetTickCount64.Call()
	now := uint64(tick64)

	// DwTime is modulo 2^32 milliseconds
	last := uint64(lii.DwTime)
	now32 := now & 0xFFFFFFFF

	var idleMillis uint64
	if now32 >= last {
		idleMillis = now32 - last
	} else {
		idleMillis = (0x100000000 - last) + now32
	}
	return time.Duration(idleMillis) * time.Millisecond, nil
}

func getMousePos() (POINT, error) {
	var p POINT
	r1, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r1 == 0 {
		return POINT{}, err
	}
	return p, nil
}

// --- Activity scoring (window) ---

type sample struct {
	t      time.Time
	active bool
}

type Mode string

const (
	ModeHighProductive   Mode = "HIGH_PRODUCTIVE"
	ModeSimpleProductive Mode = "SIMPLE_PRODUCTIVE"
	ModeIdle             Mode = "IDLE"
)

type Config struct {
	SampleEvery time.Duration
	WindowSize  time.Duration

	ActiveIfIdleLessThan time.Duration

	HighProductiveRatio   float64
	SimpleProductiveRatio float64

	ContinuousIdleThreshold time.Duration

	PrintStatusEvery    time.Duration
	PrintMouseMoveEvery time.Duration

	LogDir      string // directory where logs are stored
	LogBaseName string // e.g. "activity" => activity-YYYY-MM-DD.log

	FlushEvery time.Duration
}

// --- Daily rotating logger ---

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

	// close old
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

// --- Mode decision ---

func modeFrom(idleNow time.Duration, activeRatio float64, cfg Config) Mode {
	if idleNow >= cfg.ContinuousIdleThreshold {
		return ModeIdle
	}
	if activeRatio >= cfg.HighProductiveRatio {
		return ModeHighProductive
	}
	if activeRatio >= cfg.SimpleProductiveRatio {
		return ModeSimpleProductive
	}
	return ModeIdle
}

func main() {
	cfg := Config{
		SampleEvery:             1 * time.Second,
		WindowSize:              30 * time.Minute,
		ActiveIfIdleLessThan:    30 * time.Second,
		HighProductiveRatio:     0.60,
		SimpleProductiveRatio:   0.30,
		ContinuousIdleThreshold: 30 * time.Minute,
		PrintStatusEvery:        30 * time.Second,

		// IMPORTANT: for less “heuristic virus” behavior, consider setting this
		// to something like 10*time.Second or 1*time.Minute instead of 0.
		PrintMouseMoveEvery: 0,

		LogDir:      `C:\ProgramData\ActivityMonitor`,
		LogBaseName: "activity",

		FlushEvery: 5 * time.Second,
	}

	// Ctrl+C stop (console). When built with -H=windowsgui there is no console,
	// but this still doesn't hurt.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	rot, err := NewRotatingLogger(cfg.LogDir, cfg.LogBaseName)
	if err != nil {
		// no console in windowsgui; but during tests you'll see it
		fmt.Println("Cannot create rotating logger:", err)
		return
	}
	defer rot.Close()

	writeLine := func(line string) {
		rot.Println(line) // file only (no console)
	}

	flushTicker := time.NewTicker(cfg.FlushEvery)
	defer flushTicker.Stop()

	// init mouse position
	lastMouse, err := getMousePos()
	if err != nil {
		writeLine("GetCursorPos error: " + err.Error())
		return
	}

	var (
		samples         []sample
		lastMode        Mode = ""
		lastStatusPrint      = time.Time{}
		lastMousePrint       = time.Time{}
	)

	ticker := time.NewTicker(cfg.SampleEvery)
	defer ticker.Stop()

	writeLine(fmt.Sprintf("[%s] START (logs in %s as %s-YYYY-MM-DD.log)",
		time.Now().Format(time.RFC3339), cfg.LogDir, cfg.LogBaseName))

	for {
		select {
		case <-ctx.Done():
			writeLine(fmt.Sprintf("[%s] STOP", time.Now().Format(time.RFC3339)))
			return

		case <-flushTicker.C:
			rot.Sync()

		case now := <-ticker.C:
			ts := now.Format(time.RFC3339)

			// 1) Mouse move detection + timestamp
			p, err := getMousePos()
			if err == nil {
				if p.X != lastMouse.X || p.Y != lastMouse.Y {
					if cfg.PrintMouseMoveEvery == 0 || lastMousePrint.IsZero() || now.Sub(lastMousePrint) >= cfg.PrintMouseMoveEvery {
						dx := p.X - lastMouse.X
						dy := p.Y - lastMouse.Y
						writeLine(fmt.Sprintf("[%s] MOUSE MOVE: (%d,%d) delta=(%d,%d)", ts, p.X, p.Y, dx, dy))
						lastMousePrint = now
					}
					lastMouse = p
				}
			} else {
				writeLine(fmt.Sprintf("[%s] GetCursorPos error: %v", ts, err))
			}

			// 2) Idle + activity scoring
			idleNow, err := getIdleDuration()
			if err != nil {
				writeLine(fmt.Sprintf("[%s] GetLastInputInfo error: %v", ts, err))
				continue
			}

			isActive := idleNow < cfg.ActiveIfIdleLessThan
			samples = append(samples, sample{t: now, active: isActive})

			// Drop old samples outside window
			cutoff := now.Add(-cfg.WindowSize)
			i := 0
			for i < len(samples) && samples[i].t.Before(cutoff) {
				i++
			}
			if i > 0 {
				samples = samples[i:]
			}

			// Compute active ratio
			activeCount := 0
			for _, s := range samples {
				if s.active {
					activeCount++
				}
			}
			total := len(samples)
			activeRatio := 0.0
			if total > 0 {
				activeRatio = float64(activeCount) / float64(total)
			}

			mode := modeFrom(idleNow, activeRatio, cfg)

			// 3) Status logging
			if mode != lastMode {
				writeLine(fmt.Sprintf("[%s] MODE CHANGE: %s  idleNow=%s  activeRatio=%.0f%%  samples=%d",
					ts, mode, idleNow, activeRatio*100, total))
				lastMode = mode
				lastStatusPrint = now
				continue
			}

			if lastStatusPrint.IsZero() || now.Sub(lastStatusPrint) >= cfg.PrintStatusEvery {
				writeLine(fmt.Sprintf("[%s] STATUS: mode=%s  idleNow=%s  activeRatio=%.0f%%  samples=%d",
					ts, mode, idleNow, activeRatio*100, total))
				lastStatusPrint = now
			}
		}
	}
}
