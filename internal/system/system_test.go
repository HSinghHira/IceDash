package system

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeRoot builds the files the sampler reads, shaped like the ZimaCube in
// docs/HOST-NOTES.md: wlan0 and eth0 are physical, the rest virtual.
func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"wlan0", "eth0"} {
		mkdir(t, filepath.Join(root, "sys/class/net", name, "device"))
	}
	for _, name := range []string{"lo", "virbr0", "ztzc7zonbm", "thunderboltbr0"} {
		mkdir(t, filepath.Join(root, "sys/class/net", name))
	}
	write(t, filepath.Join(root, "proc/uptime"), "960193.10 3617460.21\n")
	write(t, filepath.Join(root, "proc/sys/kernel/hostname"), "ZimaOS\n")
	return root
}

func netDev(t *testing.T, root string, wlanRx, wlanTx, ztRx uint64) {
	t.Helper()
	write(t, filepath.Join(root, "proc/net/dev"), fmt.Sprintf(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 25330275505 19969014    0    0    0     0          0         0 25330275505 19969014    0    0    0     0       0          0
  eth0:       0       0    0    0    0     0          0         0        0       0    0    2    0     0       0          0
 wlan0: %d 36825568    0 437547    0     0          0         0 %d 27023483    0    2    0     0       0          0
virbr0:       0       0    0    0    0     0          0         0        0       0    0 121741    0     0       0          0
ztzc7zonbm: %d  113048    0    0    0     0          0         0 17081219  101470    0    0    0     0       0          0
`, wlanRx, wlanTx, ztRx))
}

func TestSpeedCountsOnlyPhysicalCards(t *testing.T) {
	root := fakeRoot(t)
	s := &Sampler{Root: root}
	t0 := time.Unix(1794381700, 0)

	netDev(t, root, 1_000_000, 500_000, 100)
	if err := s.Sample(t0); err != nil {
		t.Fatal(err)
	}
	if snap := s.Snapshot(); len(snap.History) != 0 || snap.Down != 0 {
		t.Fatalf("speed after one sample: %+v", snap)
	}

	// 2 s later: wlan0 got 4 MB and sent 1 MB; ZeroTier traffic must not count twice
	netDev(t, root, 5_000_000, 1_500_000, 9_000_100)
	if err := s.Sample(t0.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if snap.Down != 2_000_000 || snap.Up != 500_000 {
		t.Fatalf("down %v up %v, want 2000000 and 500000", snap.Down, snap.Up)
	}
	if fmt.Sprint(snap.Interfaces) != "[eth0 wlan0]" {
		t.Fatalf("interfaces %v", snap.Interfaces)
	}
	if snap.UptimeSec != 960193.10 || snap.Device != "ZimaOS" {
		t.Fatalf("uptime %v device %q", snap.UptimeSec, snap.Device)
	}
}

func TestCounterGoingBackIsNoSpike(t *testing.T) {
	root := fakeRoot(t)
	s := &Sampler{Root: root}
	t0 := time.Unix(1794381700, 0)
	netDev(t, root, 9_000_000, 9_000_000, 0)
	s.Sample(t0)
	netDev(t, root, 1_000, 1_000, 0) // the card was reset
	s.Sample(t0.Add(time.Second))
	if snap := s.Snapshot(); snap.Down != 0 || snap.Up != 0 {
		t.Fatalf("reset counter gave %v / %v", snap.Down, snap.Up)
	}
}

func TestHistoryKeepsSixtySeconds(t *testing.T) {
	root := fakeRoot(t)
	s := &Sampler{Root: root}
	t0 := time.Unix(1794381700, 0)
	for i := 0; i <= 90; i += 2 {
		netDev(t, root, uint64(i)*1000, 0, 0)
		s.Sample(t0.Add(time.Duration(i) * time.Second))
	}
	h := s.Snapshot().History
	if len(h) == 0 || h[len(h)-1].Time.Sub(h[0].Time) > Window {
		t.Fatalf("history spans %v in %d points", h[len(h)-1].Time.Sub(h[0].Time), len(h))
	}
	if len(h) != 31 {
		t.Fatalf("%d points, want 31 (every 2 s over 60 s)", len(h))
	}
}

func TestDeviceModelFromZimaOS(t *testing.T) {
	root := fakeRoot(t)
	write(t, filepath.Join(root, "run/zimaos/device-info.json"), `{"device":{"dmi":"x","model":"ZimaCube"},"cpu":{}}`)
	if d := (&Sampler{Root: root}).device(); d != "ZimaCube" {
		t.Fatalf("device %q", d)
	}
}

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func write(t *testing.T, p, s string) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
