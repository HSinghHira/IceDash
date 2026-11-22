// Package storage reads ZimaOS's list of storages, the one its Files sidebar
// and Settings → Storage show: the system disk, RAID pools, single disks and
// USB drives, each with its health and the drives in it.
//
// Measured on ZimaOS 1.8.0 (docs/HOST-NOTES.md, "Storage on ZimaOS"): local
// storage answers on the gateway, from loopback, without a token:
//
//	GET /v2/local_storage/storages  [{name, path, type, extensions:{health, size, used}}]
//	GET /v2/local_storage/raid      {data:[{name, raid_level, raid_status, percentage, devices:[…]}]}
//	GET /v2/local_storage/disk      {data:[{name, path, model, health, temperature, power_on_hours, children:[…]}]}
//
// The RAID record's fields come from ZimaOS's own Settings page (its web
// bundle); the box has no RAID to measure them on.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Pool is one storage as ZimaOS lists it.
type Pool struct {
	Name string `json:"name"`
	Dir  string `json:"dir"`  // its folder on the host: /media/<name>
	Kind string `json:"kind"` // system, raid, disk, usb (ZimaOS's type, lower case)
	Size uint64 `json:"size"`
	Used uint64 `json:"used"`
	// Health is ok, busy (a RAID resyncing, rebuilding, growing), warn
	// (degraded, a drive missing) or bad (damaged, a drive failing); "" is unknown.
	Health string `json:"health"`
	// State is a RAID's raid_status: ok, degraded, rebuilding, resyncing,
	// reshape, recovering, fail, urge, inactive, creating.
	State    string  `json:"state,omitempty"`
	Level    *int    `json:"level,omitempty"`    // RAID level; -1 is JBOD
	Progress float64 `json:"progress,omitempty"` // a resync or rebuild, in percent
	Drives   []Drive `json:"drives,omitempty"`
	// Files is the pool's top as a Files API path ("/", "<id>:/"); "" when
	// Files can't open it. Filled in by the API.
	Files string `json:"files,omitempty"`
}

// Drive is one disk in a pool.
type Drive struct {
	Name    string `json:"name"` // sdb
	Model   string `json:"model,omitempty"`
	Size    uint64 `json:"size,omitempty"`
	Health  bool   `json:"health"`
	Missing bool   `json:"missing,omitempty"`
	Temp    int    `json:"temp,omitempty"`  // °C
	Hours   int    `json:"hours,omitempty"` // power-on hours
}

// Client reads local storage through the gateway.
type Client struct {
	Gateway string // http://127.0.0.1
	HTTP    *http.Client
	TTL     time.Duration // how long an answer is kept; default 20 s

	mu    sync.Mutex
	at    time.Time
	pools []Pool
	err   error
}

// Pools lists the storages, from a short cache: Files asks on every request.
func (c *Client) Pools(ctx context.Context) ([]Pool, error) {
	ttl := c.TTL
	if ttl == 0 {
		ttl = 20 * time.Second
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.at.IsZero() && time.Since(c.at) < ttl {
		return c.pools, c.err
	}
	c.pools, c.err = c.read(ctx)
	c.at = time.Now()
	return c.pools, c.err
}

// Dirs are the pools' folders, for Files (its disk list).
func (c *Client) Dirs() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pools, _ := c.Pools(ctx)
	out := map[string]string{}
	for _, p := range pools {
		if p.Dir != "" {
			out[p.Dir] = p.Name
		}
	}
	return out
}

// num is a number that ZimaOS sometimes sends as a string ("avail":"256060514304").
type num float64

func (n *num) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil // an odd value is left at 0
	}
	*n = num(f)
	return nil
}

type rawStorage struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Extensions struct {
		Health *bool `json:"health"`
		Size   num   `json:"size"`
		Used   num   `json:"used"`
	} `json:"extensions"`
}

type rawDevice struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Model   string `json:"model"`
	Size    num    `json:"size"`
	Health  *bool  `json:"health"`
	Faulty  bool   `json:"faulty"`
	Missing bool   `json:"missing"`
}

type rawRaid struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	MountPoint  string      `json:"mount_point"`
	StorageName string      `json:"storage_name"`
	Level       *int        `json:"raid_level"`
	Status      string      `json:"raid_status"`
	Percentage  num         `json:"percentage"`
	Size        num         `json:"size"`
	Used        num         `json:"used"`
	Devices     []rawDevice `json:"devices"`
}

type rawDisk struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Model    string `json:"model"`
	Size     num    `json:"size"`
	Health   *bool  `json:"health"`
	Temp     num    `json:"temperature"`
	Hours    num    `json:"power_on_hours"`
	Children []struct {
		Path        string `json:"path"`
		MountPoint  string `json:"mount_point"`
		StorageName string `json:"storage_name"`
	} `json:"children"`
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.Gateway, "/")+"/v2/local_storage"+path, nil)
	if err != nil {
		return err
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 5 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("local storage %s answered %d", path, res.StatusCode)
	}
	return json.Unmarshal(b, out)
}

func (c *Client) read(ctx context.Context) ([]Pool, error) {
	var storages []rawStorage
	if err := c.get(ctx, "/storages", &storages); err != nil {
		return nil, err
	}
	// RAID and disk details only add to the list: without them the pools
	// still show, with less to say.
	var raids struct{ Data []rawRaid }
	_ = c.get(ctx, "/raid", &raids)
	var disks struct{ Data []rawDisk }
	_ = c.get(ctx, "/disk", &disks)
	return build(storages, raids.Data, disks.Data), nil
}

// build joins the three answers into the pools.
func build(storages []rawStorage, raids []rawRaid, disks []rawDisk) []Pool {
	byPath := map[string]*rawDisk{}
	for i := range disks {
		d := &disks[i]
		byPath[d.Path] = d
		byPath[d.Name] = d
	}
	usedRaid := map[int]bool{}
	findRaid := func(name, dir string) *rawRaid {
		for i := range raids {
			r := &raids[i]
			if usedRaid[i] {
				continue
			}
			if (name != "" && (r.Name == name || r.StorageName == name)) || (dir != "" && r.MountPoint == dir) {
				usedRaid[i] = true
				return r
			}
		}
		return nil
	}

	var out []Pool
	for _, s := range storages {
		p := Pool{Name: s.Name, Dir: s.Path, Kind: strings.ToLower(s.Type), Size: uint64(s.Extensions.Size), Used: uint64(s.Extensions.Used)}
		if h := s.Extensions.Health; h != nil {
			p.Health = map[bool]string{true: "ok", false: "bad"}[*h]
		}
		if r := findRaid(s.Name, s.Path); r != nil {
			raidInto(&p, r, byPath)
		} else {
			p.Drives = drivesOf(p, disks)
		}
		out = append(out, p)
	}
	// A RAID ZimaOS has but doesn't list as a storage (not mounted yet).
	for i := range raids {
		if usedRaid[i] {
			continue
		}
		r := &raids[i]
		p := Pool{Name: r.Name, Dir: r.MountPoint, Kind: "raid", Size: uint64(r.Size), Used: uint64(r.Used)}
		raidInto(&p, r, byPath)
		out = append(out, p)
	}
	for i := range out {
		for _, d := range out[i].Drives {
			switch {
			case d.Missing:
				out[i].Health = worst(out[i].Health, "warn")
			case !d.Health:
				out[i].Health = worst(out[i].Health, "bad")
			}
		}
	}
	return out
}

func raidInto(p *Pool, r *rawRaid, byPath map[string]*rawDisk) {
	p.Kind = "raid"
	p.Level = r.Level
	p.State = r.Status
	if p.State == "" {
		p.State = "ok" // as ZimaOS's page reads it
	}
	p.Health = worst(p.Health, raidHealth(p.State))
	if p.State != "ok" && r.Percentage > 0 && r.Percentage < 100 {
		p.Progress = float64(r.Percentage)
	}
	if p.Size == 0 {
		p.Size, p.Used = uint64(r.Size), uint64(r.Used)
	}
	for _, d := range r.Devices {
		dr := Drive{Name: d.Name, Model: d.Model, Size: uint64(d.Size), Health: (d.Health == nil || *d.Health) && !d.Faulty, Missing: d.Missing}
		if dr.Name == "" {
			dr.Name = strings.TrimPrefix(d.Path, "/dev/")
		}
		if k := byPath[d.Path]; k != nil {
			fill(&dr, k)
		} else if k := byPath[dr.Name]; k != nil {
			fill(&dr, k)
		}
		p.Drives = append(p.Drives, dr)
	}
}

// drivesOf finds the disks a storage that isn't a RAID lives on: those with
// a partition mounted at its folder or named after it. The system storage
// (ZimaOS-HD, which is /DATA) is on the disk ZimaOS calls "System".
func drivesOf(p Pool, disks []rawDisk) []Drive {
	var out []Drive
	for i := range disks {
		d := &disks[i]
		on := p.Kind == "system" && d.Model == "System"
		for _, ch := range d.Children {
			if ch.MountPoint != "" && (ch.MountPoint == p.Dir || (p.Kind == "system" && ch.MountPoint == "/DATA")) || ch.StorageName != "" && ch.StorageName == p.Name {
				on = true
			}
		}
		if on {
			dr := Drive{Health: true}
			fill(&dr, d)
			out = append(out, dr)
		}
	}
	return out
}

func fill(dr *Drive, d *rawDisk) {
	if dr.Name == "" {
		dr.Name = d.Name
	}
	if d.Model != "" && d.Model != "System" {
		dr.Model = d.Model
	}
	if d.Size > 0 {
		dr.Size = uint64(d.Size)
	}
	if d.Health != nil && !*d.Health {
		dr.Health = false
	}
	dr.Temp, dr.Hours = int(d.Temp), int(d.Hours)
}

// raidHealth sorts ZimaOS's RAID states the way its Settings page colours
// them: green ok, blue busy, yellow degraded, red failed, grey inactive.
func raidHealth(state string) string {
	switch state {
	case "ok":
		return "ok"
	case "rebuilding", "resyncing", "reshape", "recovering", "creating":
		return "busy"
	case "degraded", "inactive":
		return "warn"
	case "fail", "urge":
		return "bad"
	}
	return ""
}

var rank = map[string]int{"": 0, "ok": 1, "busy": 2, "warn": 3, "bad": 4}

// worst keeps the more worrying of two healths.
func worst(a, b string) string {
	if rank[b] > rank[a] {
		return b
	}
	return a
}
