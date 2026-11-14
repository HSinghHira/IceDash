package update

import (
	"context"
	"time"

	"github.com/HSinghHira/IceDash/internal/notify"
)

// The night window (PLAN.md 5a.1): an update that waits installs between
// 02:00 and 05:00 in the zone saved with the setting, never the box's own
// zone (on the measured ZimaOS box that is Europe/London for a user in
// Pacific/Auckland).
const (
	NightStart = 2
	NightEnd   = 5
)

// NightDue reports whether now falls in the night window in zone, and which
// night it is (the date in that zone), so each night installs at most once.
func NightDue(now time.Time, zone string) (bool, string) {
	if !ValidZone(zone) {
		return false, ""
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return false, ""
	}
	t := now.In(loc)
	return t.Hour() >= NightStart && t.Hour() < NightEnd, t.Format(time.DateOnly)
}

// NightLoop installs a waiting update at night while on() says so: once a
// night it checks GitHub, and if a newer release is there it installs it,
// with the usual rollback.
func (m *Manager) NightLoop(ctx context.Context, on func() (bool, string), now func() time.Time) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	done := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			done = m.nightStep(ctx, on, now(), done)
		}
	}
}

// nightStep is one minute of NightLoop; it returns the night already handled.
func (m *Manager) nightStep(ctx context.Context, on func() (bool, string), now time.Time, done string) string {
	enabled, zone := on()
	if !enabled {
		return done
	}
	due, night := NightDue(now, zone)
	if !due || night == done {
		return done
	}
	cctx, cancel := context.WithTimeout(ctx, time.Minute)
	st := m.Check(cctx)
	cancel()
	if st.State != StateAvailable {
		// GitHub didn't answer: try again next minute. Nothing newer: done tonight.
		if st.Error != "" {
			return done
		}
		return night
	}
	m.notify(notify.Notification{Kind: notify.KindNightInstall, Version: st.Latest,
		Title: "Installing " + st.Latest + " overnight",
		Body:  "IceDash is installing " + st.Latest + " now, as set in Settings. It comes back by itself, or goes back to the previous version if anything fails."})
	_ = m.Apply() // its result is notified as for Update now
	return night
}

// ValidZone reports whether zone is a real IANA name (not "", "Local" or "UTC"
// spelled any other way than a known name).
func ValidZone(zone string) bool {
	if zone == "" || zone == "Local" || len(zone) > 64 {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}
