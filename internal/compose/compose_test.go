package compose

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// Folded text comes out as a literal block with the same value: ZimaOS can't
// build edits on an installed file with folded text (HOST-NOTES, 2026-11-17).
func TestBytesUnfolds(t *testing.T) {
	in := "name: a\nservices:\n  a:\n    image: x\nx-casaos:\n  description:\n    en_US: >-\n      One line\n      goes on.\n\n\n      - a list\n        item\n"
	d, err := Parse([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := str(get(get(d.X(), "description"), "en_US"))
	b, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), ">-") || !strings.Contains(string(b), "en_US: |-") {
		t.Errorf("still folded:\n%s", b)
	}
	d2, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := str(get(get(d2.X(), "description"), "en_US")); got != want {
		t.Errorf("value changed:\n%q\n%q", got, want)
	}
}

func load(t *testing.T, name string) *Doc {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func input(s Setup, key string) Input {
	for _, in := range s.Inputs {
		if in.Key == key {
			return in
		}
	}
	return Input{}
}

func TestParseRejectsNonCompose(t *testing.T) {
	for _, b := range []string{"", "- a\n- b\n", "name: x\n", "services: {}\n", ": :"} {
		if _, err := Parse([]byte(b)); err == nil {
			t.Errorf("%q: want an error", b)
		}
	}
}

func TestDocumensoSetup(t *testing.T) {
	d := load(t, "documenso.yml")
	if d.Name() != "documenso" || d.Main() != "documenso" || d.Container() != "documenso" {
		t.Fatalf("name %q main %q container %q", d.Name(), d.Main(), d.Container())
	}
	s := d.Setup(Context{Host: "192.168.68.17", Email: "admin@home.lan"})
	if s.Address != "http://192.168.68.17:6142" {
		t.Errorf("address %q", s.Address)
	}
	u := input(s, "PUBLIC_URL")
	if u.Value != "http://192.168.68.17:6142" || u.Fill != FillAuto || u.Required || !u.Main {
		t.Errorf("PUBLIC_URL %+v", u)
	}
	if !strings.Contains(u.Description, "address you open Documenso at") {
		t.Errorf("PUBLIC_URL description %q", u.Description)
	}
	e := input(s, "DOCUMENSO_ADMIN_EMAIL")
	if e.Value != "admin@home.lan" || e.Fill != FillAccount || !e.Main {
		t.Errorf("admin email %+v", e)
	}
	if tz := input(s, "TZ"); tz.Main || tz.Value != "$TZ" {
		t.Errorf("TZ %+v", tz)
	}
	if len(s.Ports) != 1 || s.Ports[0].Published != "6142" || s.Ports[0].Target != "8080" {
		t.Errorf("ports %+v", s.Ports)
	}
	if !s.Access.Privileged || s.Access.DockerSocket || s.Data != "/DATA/AppData/documenso" {
		t.Errorf("access %+v data %q", s.Access, s.Data)
	}

	// Without an account email the field stays empty and optional.
	s = d.Setup(Context{Host: "192.168.68.17"})
	if e := input(s, "DOCUMENSO_ADMIN_EMAIL"); e.Value != "" || e.Fill != "" || e.Required || !e.Main {
		t.Errorf("admin email without account %+v", e)
	}
}

func TestAutoXposeNeedsThePublicIP(t *testing.T) {
	d := load(t, "autoxpose.yml")
	s := d.Setup(Context{Host: "192.168.68.17"})
	if lan := input(s, "LAN_IP"); lan.Value != "192.168.68.17" || lan.Fill != FillAuto || lan.Required {
		t.Errorf("LAN_IP %+v", lan)
	}
	pub := input(s, "SERVER_IP")
	if pub.Value != "CHANGEME_PUBLIC_IP" || !pub.Required || !pub.Main || pub.Fill != "" {
		t.Errorf("SERVER_IP %+v", pub)
	}
	if !s.Access.DockerSocket || s.Access.Privileged || s.Access.HostNetwork {
		t.Errorf("access %+v", s.Access)
	}

	// Apply refuses while the public IP is missing...
	err := d.Apply(Choices{Values: map[string]string{"autoxpose/LAN_IP": "192.168.68.17"}})
	var miss *MissingError
	if !errors.As(err, &miss) || len(miss.Keys) != 1 || miss.Keys[0] != "SERVER_IP" {
		t.Fatalf("want SERVER_IP missing, got %v", err)
	}
	// ...and takes it, with a moved port.
	err = d.Apply(Choices{
		Values: map[string]string{"autoxpose/SERVER_IP": "203.0.113.1"},
		Ports:  map[string]string{"autoxpose/3000/tcp": "6199"},
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range d2.Envs() {
		got[e.Key] = e.Value
	}
	if got["SERVER_IP"] != "203.0.113.1" || got["LAN_IP"] != "192.168.68.17" || got["TZ"] != "$TZ" {
		t.Errorf("envs after apply %v", got)
	}
	if p := d2.Ports(); p[0].Published != "6199" || d2.XString("port_map") != "6199" {
		t.Errorf("port %+v port_map %q", p, d2.XString("port_map"))
	}
	// Descriptions and the rest survive.
	if !strings.Contains(string(out), "Read-only access to Docker") || !strings.Contains(string(out), "release_notes") {
		t.Error("lost parts of the file")
	}
}

func TestApplyRejectsUnknownAndBad(t *testing.T) {
	cases := []Choices{
		{Values: map[string]string{"autoxpose/NOPE": "x"}},
		{Values: map[string]string{"nosvc/TZ": "x"}},
		{Values: map[string]string{"autoxpose/TZ": "a\nb"}},
		{Ports: map[string]string{"autoxpose/3000/tcp": "0"}},
		{Ports: map[string]string{"autoxpose/3000/tcp": "70000"}},
		{Ports: map[string]string{"autoxpose/9999/tcp": "6200"}},
	}
	for i, c := range cases {
		if err := load(t, "autoxpose.yml").Apply(c); err == nil {
			t.Errorf("case %d: want an error", i)
		}
	}
}

func TestPlaceholdersAndGenerated(t *testing.T) {
	d, err := Parse([]byte(`name: demo
services:
  web:
    image: x
    environment:
      ORIGIN: CHANGEME_ORIGIN
      RP_ID: CHANGEME_RP_ID
      DB_PASSWORD: CHANGEME_PASSWORD
      OIDC_CLIENT_ID: CHANGEME_OIDC_CLIENT_ID
      PLAIN: CHANGEME
      HOST: http://[ZIMAOS-IP-ADDRESS]:6150/app
      OPTIONAL: ""
    ports:
      - "6150:80"
x-casaos:
  port_map: "6150"
  index: /app
`))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Setup(Context{Host: "10.0.0.5", Secret: func() string { return "s3cret" }})
	want := map[string]struct {
		v        string
		fill     string
		required bool
	}{
		"ORIGIN":         {"http://10.0.0.5:6150", FillAuto, false},
		"RP_ID":          {"10.0.0.5", FillAuto, false},
		"DB_PASSWORD":    {"s3cret", FillGenerated, false},
		"OIDC_CLIENT_ID": {"CHANGEME_OIDC_CLIENT_ID", "", true},
		"PLAIN":          {"CHANGEME", "", true},
		"HOST":           {"http://10.0.0.5:6150/app", FillAuto, false},
		"OPTIONAL":       {"", "", false},
	}
	for k, w := range want {
		in := input(s, k)
		if in.Value != w.v || in.Fill != w.fill || in.Required != w.required {
			t.Errorf("%s: %+v, want %+v", k, in, w)
		}
	}
	if !input(s, "DB_PASSWORD").Secret || input(s, "OPTIONAL").Main {
		t.Error("secret or main marks wrong")
	}
	if s.Address != "http://10.0.0.5:6150/app" {
		t.Errorf("address %q", s.Address)
	}
	// Short port form moves too.
	if !d.SetPublished("web", "80", "tcp", "6151") || d.Ports()[0].Published != "6151" || d.XString("port_map") != "6151" {
		t.Errorf("short port: %+v", d.Ports())
	}
}

func TestKeepFromCarriesSettings(t *testing.T) {
	old := load(t, "autoxpose.yml")
	if err := old.Apply(Choices{
		Values: map[string]string{"autoxpose/SERVER_IP": "203.0.113.9", "autoxpose/LAN_IP": "192.168.68.17"},
		Ports:  map[string]string{"autoxpose/3000/tcp": "6111"},
	}); err != nil {
		t.Fatal(err)
	}
	newer := load(t, "autoxpose.yml")
	newer.KeepFrom(old)
	got := map[string]string{}
	for _, e := range newer.Envs() {
		got[e.Key] = e.Value
	}
	if got["SERVER_IP"] != "203.0.113.9" || newer.Ports()[0].Published != "6111" {
		t.Errorf("kept %v ports %+v", got, newer.Ports())
	}
}

func TestHostOnly(t *testing.T) {
	for in, want := range map[string]string{"192.168.68.17": "192.168.68.17", "192.168.68.17:80": "192.168.68.17", "[fe80::1]:80": "fe80::1", "zima.local": "zima.local"} {
		if got := HostOnly(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

// An app on the host network has no ports; the setup must still send [] for
// ports and inputs, not null, or the setup popup can't open.
func TestSetupHostNetworkSendsEmptyLists(t *testing.T) {
	d, err := Parse([]byte(`name: lan
services:
  lan:
    image: example/lan:1
    network_mode: host
x-casaos:
  port_map: "8840"
`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(d.Setup(Context{Host: "10.0.0.5"}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"ports":[]`, `"inputs":[]`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("setup %s: want %s", b, want)
		}
	}
}

func TestFolders(t *testing.T) {
	d := load(t, "autoxpose.yml")
	vs := d.Volumes()
	// The Docker socket is a system mount, not one of the user's folders.
	if len(vs) != 1 || vs[0].Source != "/DATA/AppData/$AppID/data" || vs[0].Target != "/app/packages/backend/data" ||
		vs[0].Service != "autoxpose" || !strings.Contains(vs[0].Description, "SQLite") {
		t.Fatalf("volumes %+v", vs)
	}
	if s := d.Setup(Context{Host: "h"}); len(s.Folders) != 1 {
		t.Errorf("setup folders %+v", s.Folders)
	}
	ch := Choices{
		Values:  map[string]string{"autoxpose/SERVER_IP": "1.2.3.4", "autoxpose/LAN_IP": "10.0.0.2"},
		Folders: map[string]string{"autoxpose//app/packages/backend/data": "/DATA/AppData/autoxpose/db"},
		Add:     []NewFolder{{Source: "/media/MEDIAS/SERIES-TV", Target: "/series"}},
	}
	if err := d.Apply(ch); err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, v := range d.Volumes() {
		got[v.Target] = v.Source
	}
	if got["/app/packages/backend/data"] != "/DATA/AppData/autoxpose/db" || got["/series"] != "/media/MEDIAS/SERIES-TV" {
		t.Errorf("after apply %v", got)
	}
	b, _ := d.Bytes()
	if !strings.Contains(string(b), "source: /media/MEDIAS/SERIES-TV\n        target: /series") {
		t.Errorf("added bind not in the file:\n%s", b)
	}
}

func TestFoldersRefused(t *testing.T) {
	vals := map[string]string{"autoxpose/SERVER_IP": "1.2.3.4", "autoxpose/LAN_IP": "10.0.0.2"}
	key := "autoxpose//app/packages/backend/data"
	for name, ch := range map[string]Choices{
		"outside roots": {Values: vals, Folders: map[string]string{key: "/etc"}},
		"climbs out":    {Values: vals, Folders: map[string]string{key: "/DATA/../etc"}},
		"unknown":       {Values: vals, Folders: map[string]string{"autoxpose//nope": "/DATA/x"}},
		"two lines":     {Values: vals, Add: []NewFolder{{Source: "/DATA/a\nb", Target: "/a"}}},
		"root target":   {Values: vals, Add: []NewFolder{{Source: "/DATA/a", Target: "/"}}},
		"taken target":  {Values: vals, Add: []NewFolder{{Source: "/DATA/a", Target: "/var/run/docker.sock"}}},
		"no service":    {Values: vals, Add: []NewFolder{{Service: "db", Source: "/DATA/a", Target: "/a"}}},
	} {
		if err := load(t, "autoxpose.yml").Apply(ch); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A folder left as it was passes, even with ZimaOS's $AppID in it.
	ok := Choices{Values: vals, Folders: map[string]string{key: "/DATA/AppData/$AppID/data"}}
	if err := load(t, "autoxpose.yml").Apply(ok); err != nil {
		t.Errorf("unchanged folder: %v", err)
	}
}

func TestShortFolders(t *testing.T) {
	d, err := Parse([]byte("name: a\nservices:\n  a:\n    image: x\n    volumes:\n      - /DATA/Media:/media:ro\n      - data:/var/lib/a\n      - /etc/localtime:/etc/localtime:ro\n"))
	if err != nil {
		t.Fatal(err)
	}
	vs := d.Volumes()
	if len(vs) != 1 || vs[0].Source != "/DATA/Media" || !vs[0].ReadOnly {
		t.Fatalf("%+v", vs)
	}
	if !d.SetSource("a", "/media", "/media/USB") {
		t.Fatal("not set")
	}
	b, _ := d.Bytes()
	if !strings.Contains(string(b), "- /media/USB:/media:ro") {
		t.Errorf("%s", b)
	}
}

// An update keeps the folders the user moved or added (ZimaOS reports the
// installed file in its long JSON form).
func TestKeepFromFolders(t *testing.T) {
	old, err := Parse([]byte(`{"name":"autoxpose","services":{"autoxpose":{"image":"x","volumes":[
		{"type":"bind","source":"/DATA/AppData/autoxpose/db","target":"/app/packages/backend/data"},
		{"type":"bind","source":"/var/run/docker.sock","target":"/var/run/docker.sock"},
		{"type":"bind","source":"/media/MEDIAS/SERIES-TV","target":"/series"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	d := load(t, "autoxpose.yml")
	d.KeepFrom(old)
	got := map[string]string{}
	for _, v := range d.Volumes() {
		got[v.Target] = v.Source
	}
	if len(got) != 2 || got["/app/packages/backend/data"] != "/DATA/AppData/autoxpose/db" || got["/series"] != "/media/MEDIAS/SERIES-TV" {
		t.Errorf("%v", got)
	}
}
