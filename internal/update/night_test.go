package update

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/HSinghHira/IceDash/internal/notify"
)

func TestNightDueUsesTheSavedZone(t *testing.T) {
	// 14:30 UTC is 03:30 the next day in Auckland (NZDT, UTC+13) and 14:30 in London.
	now := time.Date(2026, 11, 14, 14, 30, 0, 0, time.UTC)
	if due, night := NightDue(now, "Pacific/Auckland"); !due || night != "2026-11-15" {
		t.Fatalf("Auckland: %v %s", due, night)
	}
	if due, _ := NightDue(now, "Europe/London"); due {
		t.Fatal("London at 14:30 isn't night")
	}
	for _, z := range []string{"", "Mars/Olympus", "Local"} {
		if due, _ := NightDue(now, z); due {
			t.Fatalf("zone %q counted", z)
		}
	}
	edge := time.Date(2026, 11, 14, 16, 0, 0, 0, time.UTC) // 05:00 in Auckland: the window has closed
	if due, _ := NightDue(edge, "Pacific/Auckland"); due {
		t.Fatal("05:00 is outside the window")
	}
}

func TestNightStepInstallsOncePerNight(t *testing.T) {
	f := newFake(t, "v0.2.0")
	m, n, _ := manager(t, f)
	ctx := context.Background()
	on := func() (bool, string) { return true, "Pacific/Auckland" }
	day := time.Date(2026, 11, 14, 1, 0, 0, 0, time.UTC) // 14:00 in Auckland
	if got := m.nightStep(ctx, on, day, ""); got != "" || m.Status().State != StateIdle {
		t.Fatalf("daytime: %q %s", got, m.Status().State)
	}
	off := func() (bool, string) { return false, "Pacific/Auckland" }
	night := time.Date(2026, 11, 14, 14, 30, 0, 0, time.UTC)
	if got := m.nightStep(ctx, off, night, ""); got != "" || m.Status().State != StateIdle {
		t.Fatal("switched off, but it ran")
	}
	got := m.nightStep(ctx, on, night, "")
	if got != "2026-11-15" {
		t.Fatalf("night %q", got)
	}
	if st := m.Status(); st.State != StateInstalling && st.State != StateIdle && st.State != StateFailed {
		t.Fatalf("state %s", st.State)
	}
	if !slices.Contains(n.kinds(), notify.KindNightInstall) {
		t.Fatalf("notifications %v", n.kinds())
	}
	// The install runs on in the background and raises its own result, so
	// count only the overnight notices.
	nights := func() int {
		return len(slices.DeleteFunc(n.kinds(), func(k string) bool { return k != notify.KindNightInstall }))
	}
	before := nights()
	if again := m.nightStep(ctx, on, night.Add(time.Minute), got); again != got || nights() != before {
		t.Fatal("ran twice in one night")
	}
	// The install goes on in the background: let it reach the handover (where
	// a real box restarts) or fail, before the test's folders go.
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		st := m.Status()
		if n := len(st.Steps); st.State != StateInstalling || n > 0 && st.Steps[n-1].ID == "handover" {
			break
		}
	}
}
