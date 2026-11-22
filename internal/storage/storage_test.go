package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The system disk as measured on ZimaOS 1.8.0, plus a two-disk RAID 1 pool
// that has lost a drive and a single btrfs disk (shapes from ZimaOS's
// Settings page; no RAID on the box to measure).
const (
	storagesJSON = `[
{"extensions":{"health":true,"size":238030327808,"used":130440863744},"name":"ZimaOS-HD","path":"/media/ZimaOS-HD","type":"SYSTEM"},
{"extensions":{"health":true,"size":4000000000000,"used":1000000000000},"name":"Pool","path":"/media/Pool","type":"RAID"},
{"extensions":{"health":true,"size":"2000000000000","used":"500"},"name":"Backup","path":"/media/Backup","type":"DISK"}]`
	raidJSON = `{"data":[
{"name":"Pool","raid_level":1,"raid_status":"degraded","percentage":0,"disk_number":2,
 "devices":[{"path":"/dev/sdb","model":"WD Red","health":true,"index":1},{"path":"","model":"","missing":true,"index":2}]},
{"name":"Fresh","raid_level":5,"raid_status":"resyncing","percentage":"42.5","mount_point":"/media/Fresh","devices":[]}],"message":"success"}`
	diskJSON = `{"data":[
{"avail":"256060514304","model":"System","name":"sda","path":"/dev/sda","health":true,"temperature":34,"power_on_hours":18686,"size":256060514304,
 "children":[{"mount_point":"/DATA","name":"sda8","path":"/dev/sda8","storage_name":"DATA"}]},
{"model":"WD Red","name":"sdb","path":"/dev/sdb","health":true,"temperature":38,"power_on_hours":900,"size":4000000000000,"children":[]},
{"model":"Seagate","name":"sdc","path":"/dev/sdc","health":false,"temperature":51,"size":2000000000000,
 "children":[{"mount_point":"/media/Backup","path":"/dev/sdc1","storage_name":"Backup"}]}],"message":"success"}`
)

func TestPools(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/v2/local_storage/storages":
			w.Write([]byte(storagesJSON))
		case "/v2/local_storage/raid":
			w.Write([]byte(raidJSON))
		case "/v2/local_storage/disk":
			w.Write([]byte(diskJSON))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{Gateway: srv.URL}
	pools, err := c.Pools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) != 4 {
		t.Fatalf("pools %+v", pools)
	}
	sys, pool, backup, fresh := pools[0], pools[1], pools[2], pools[3]
	if sys.Kind != "system" || sys.Health != "ok" || len(sys.Drives) != 1 || sys.Drives[0].Temp != 34 || sys.Drives[0].Model != "" {
		t.Errorf("system %+v", sys)
	}
	if pool.Kind != "raid" || *pool.Level != 1 || pool.State != "degraded" || pool.Health != "warn" || len(pool.Drives) != 2 {
		t.Errorf("pool %+v", pool)
	}
	if d := pool.Drives[0]; d.Name != "sdb" || d.Temp != 38 || !d.Health || d.Hours != 900 {
		t.Errorf("pool drive %+v", d)
	}
	if d := pool.Drives[1]; !d.Missing {
		t.Errorf("missing drive %+v", d)
	}
	if backup.Kind != "disk" || backup.Used != 500 || backup.Health != "bad" || len(backup.Drives) != 1 || backup.Drives[0].Health {
		t.Errorf("backup %+v", backup)
	}
	if fresh.Dir != "/media/Fresh" || fresh.Health != "busy" || fresh.Progress != 42.5 {
		t.Errorf("fresh %+v", fresh)
	}
	if d := c.Dirs(); d["/media/Pool"] != "Pool" || len(d) != 4 {
		t.Errorf("dirs %v", d)
	}
	if calls != 3 {
		t.Errorf("%d calls, want 3 (cached)", calls)
	}
}

// Off ZimaOS (or with local storage down) there are no pools and an error.
func TestPoolsUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := &Client{Gateway: srv.URL}
	if p, err := c.Pools(context.Background()); err == nil || len(p) != 0 {
		t.Fatalf("got %v, %v", p, err)
	}
	if len(c.Dirs()) != 0 {
		t.Fatal("dirs without pools")
	}
}
