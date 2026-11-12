package compose

import (
	"errors"
	"os"
	"strings"
	"testing"
)

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
