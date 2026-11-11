// Package system measures what ZimaOS's /v1/sys/utilization lacks: network
// speed (it only gives running byte totals), uptime and the device name.
// CPU, memory and /DATA storage come from ZimaOS itself (docs/HOST-NOTES.md item 2).
package system

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Point is the network speed at one moment, in bytes per second.
type Point struct {
	Time time.Time `json:"time"`
	Down float64   `json:"down"`
	Up   float64   `json:"up"`
}

// Snapshot is the answer of GET /v2/icedash/system.
type Snapshot struct {
	Down       float64  `json:"down"` // bytes per second
	Up         float64  `json:"up"`
	History    []Point  `json:"history"` // the last 60 s, oldest first
	Interfaces []string `json:"interfaces"`
	UptimeSec  float64  `json:"uptime_sec"`
	Device     string   `json:"device"`
}

// Window is how much speed history a Snapshot carries (the sparkline).
const Window = 60 * time.Second

// Sampler reads /proc/net/dev at a steady pace and keeps the speed history.
type Sampler struct {
	// Root is prefixed to every path read ("/" on the box, a folder in tests).
	Root string

	mu      sync.Mutex
	last    map[string]counters
	lastAt  time.Time
	history []Point
	ifaces  []string
}

type counters struct{ rx, tx uint64 }

// New returns a Sampler reading from the real /proc, /sys and /run.
func New() *Sampler { return &Sampler{Root: "/"} }

func (s *Sampler) path(p string) string { return filepath.Join(s.Root, p) }

// Run samples every interval until ctx ends.
func (s *Sampler) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	_ = s.Sample(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			_ = s.Sample(now)
		}
	}
}

// Sample reads the counters once and, from the second call on, adds a speed point.
func (s *Sampler) Sample(now time.Time) error {
	all, err := s.readNetDev()
	if err != nil {
		return err
	}
	cur := map[string]counters{}
	var names []string
	for _, name := range s.realInterfaces(all) {
		cur[name] = all[name]
		names = append(names, name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last != nil {
		dt := now.Sub(s.lastAt).Seconds()
		if dt > 0 {
			var down, up float64
			for name, c := range cur {
				p, ok := s.last[name]
				if !ok {
					continue
				}
				down += delta(p.rx, c.rx) / dt
				up += delta(p.tx, c.tx) / dt
			}
			s.history = append(s.history, Point{Time: now, Down: down, Up: up})
		}
	}
	cut := now.Add(-Window)
	i := 0
	for i < len(s.history) && s.history[i].Time.Before(cut) {
		i++
	}
	s.history = s.history[i:]
	s.last, s.lastAt, s.ifaces = cur, now, names
	return nil
}

// delta is how far a counter moved; a counter that went back (an interface
// came back up, or wrapped) counts as no traffic rather than a huge spike.
func delta(prev, cur uint64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur - prev)
}

// Snapshot returns the current speed, history, uptime and device name.
func (s *Sampler) Snapshot() Snapshot {
	s.mu.Lock()
	snap := Snapshot{
		History:    append([]Point{}, s.history...),
		Interfaces: append([]string{}, s.ifaces...),
	}
	if n := len(s.history); n > 0 {
		snap.Down, snap.Up = s.history[n-1].Down, s.history[n-1].Up
	}
	s.mu.Unlock()
	snap.UptimeSec = s.uptime()
	snap.Device = s.device()
	return snap
}

// readNetDev parses /proc/net/dev: "  wlan0: <rx bytes> … <tx bytes> …".
func (s *Sampler) readNetDev() (map[string]counters, error) {
	f, err := os.Open(s.path("proc/net/dev"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]counters{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		rx, err1 := strconv.ParseUint(fields[0], 10, 64)
		tx, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out[strings.TrimSpace(name)] = counters{rx: rx, tx: tx}
	}
	return out, sc.Err()
}

// realInterfaces keeps the physical network cards: those with a
// /sys/class/net/<name>/device link. That drops lo, Docker and VM bridges,
// veth pairs and VPNs such as ZeroTier, which would count traffic twice.
// Without any physical card it falls back to everything but lo.
func (s *Sampler) realInterfaces(all map[string]counters) []string {
	var real, rest []string
	for name := range all {
		if name == "lo" {
			continue
		}
		if _, err := os.Stat(s.path(filepath.Join("sys/class/net", name, "device"))); err == nil {
			real = append(real, name)
		} else {
			rest = append(rest, name)
		}
	}
	if len(real) == 0 {
		real = rest
	}
	slices.Sort(real)
	return real
}

func (s *Sampler) uptime() float64 {
	b, err := os.ReadFile(s.path("proc/uptime"))
	if err != nil {
		return 0
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(b)), " ")
	v, _ := strconv.ParseFloat(first, 64)
	return v
}

// device is the model from ZimaOS's /run/zimaos/device-info.json, else the hostname.
func (s *Sampler) device() string {
	if b, err := os.ReadFile(s.path("run/zimaos/device-info.json")); err == nil {
		var info struct {
			Device struct {
				Model string `json:"model"`
			} `json:"device"`
		}
		if json.Unmarshal(b, &info) == nil {
			if m := strings.TrimSpace(info.Device.Model); m != "" {
				return m
			}
		}
	}
	if b, err := os.ReadFile(s.path("proc/sys/kernel/hostname")); err == nil {
		return strings.TrimSpace(string(b))
	}
	return ""
}
