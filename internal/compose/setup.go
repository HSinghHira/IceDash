package compose

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
)

// Placeholders IceStore leaves in env values for the user (PLAN.md, Phase 2,
// measured on 2026-11-12 over all 122 apps).
var (
	ipMarks     = []string{"[ZIMAOS-IP-ADDRESS]", "[SERVER-IP]"}
	changeMe    = regexp.MustCompile(`CHANGE_?ME[A-Z0-9_]*`)
	emailKey    = regexp.MustCompile(`(^|_)E?MAIL$`)
	adminKey    = regexp.MustCompile(`ADMIN`)
	secretWords = regexp.MustCompile(`PASSWORD|PASSWD|SECRET|TOKEN|_KEY$|^KEY$`)
)

// HasPlaceholder reports whether a value still needs the user.
func HasPlaceholder(v string) bool {
	for _, m := range ipMarks {
		if strings.Contains(v, m) {
			return true
		}
	}
	return changeMe.MatchString(v)
}

// Context is what IceDash knows to fill in.
type Context struct {
	Host  string // the host the user opened IceDash at (an IP or a name)
	Email string // the ZimaOS account's email, often empty
	// Secret makes a new random value; nil uses crypto/rand.
	Secret func() string
}

// Fill sources, shown as marks in the setup popup.
const (
	FillAuto      = "auto"      // the server's address
	FillAccount   = "account"   // from the ZimaOS account
	FillGenerated = "generated" // a new random secret
)

// Input is one env value in the setup popup.
type Input struct {
	Env
	Fill     string `json:"fill,omitempty"`
	Required bool   `json:"required"` // still has a placeholder only the user can replace
	Main     bool   `json:"main"`     // shown first; the rest go under Advanced
	Secret   bool   `json:"secret"`
}

// Setup is everything the setup popup shows before an install.
type Setup struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Inputs  []Input `json:"inputs"`
	Ports   []Port  `json:"ports"`
	Data    string  `json:"data"`
	Access  Access  `json:"access"`
}

// Address is where the app will answer: scheme://host:port_map/index.
func (d *Doc) Address(host string) string {
	port := d.XString("port_map")
	if port == "" {
		if ps := d.Ports(); len(ps) > 0 {
			port = ps[0].Published
		}
	}
	scheme := d.XString("scheme")
	if scheme != "https" {
		scheme = "http"
	}
	index := d.XString("index")
	if index == "" || index == "/" {
		index = ""
	} else if !strings.HasPrefix(index, "/") {
		index = "/" + index
	}
	h := host
	if strings.Contains(h, ":") && !strings.HasPrefix(h, "[") {
		h = "[" + h + "]"
	}
	if port == "" {
		return scheme + "://" + h + index
	}
	return scheme + "://" + h + ":" + port + index
}

// Setup works out the popup's fields with what IceDash can fill in.
func (d *Doc) Setup(c Context) Setup {
	secret := c.Secret
	if secret == nil {
		secret = NewSecret
	}
	origin := strings.TrimSuffix(d.Address(c.Host), d.indexPath())
	host := c.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}

	s := Setup{Name: d.Name(), Address: d.Address(c.Host), Ports: d.Ports(), Data: "/DATA/AppData/" + d.Name(), Access: d.Access()}
	for _, e := range d.Envs() {
		in := Input{Env: e, Secret: secretWords.MatchString(e.Key)}
		v := e.Value
		for _, m := range ipMarks {
			if strings.Contains(v, m) {
				v = strings.ReplaceAll(v, m, host)
				in.Fill = FillAuto
			}
		}
		switch {
		case strings.Contains(v, "CHANGEME_LAN_IP"):
			v, in.Fill = strings.ReplaceAll(v, "CHANGEME_LAN_IP", c.Host), FillAuto
		case strings.Contains(v, "CHANGEME_RP_ID"):
			v, in.Fill = strings.ReplaceAll(v, "CHANGEME_RP_ID", c.Host), FillAuto
		case strings.Contains(v, "CHANGEME_ORIGIN"):
			v, in.Fill = strings.ReplaceAll(v, "CHANGEME_ORIGIN", origin), FillAuto
		case strings.Contains(v, "CHANGEME_PASSWORD"):
			v, in.Fill, in.Secret = strings.ReplaceAll(v, "CHANGEME_PASSWORD", secret()), FillGenerated, true
		case v == "" && c.Email != "" && emailKey.MatchString(e.Key) && adminKey.MatchString(e.Key):
			v, in.Fill = c.Email, FillAccount
		}
		in.Value = v
		in.Required = HasPlaceholder(v)
		in.Main = in.Fill != "" || in.Required || HasPlaceholder(e.Value) ||
			(emailKey.MatchString(e.Key) && adminKey.MatchString(e.Key))
		s.Inputs = append(s.Inputs, in)
	}
	return s
}

func (d *Doc) indexPath() string {
	i := d.XString("index")
	if i == "" || i == "/" {
		return ""
	}
	if !strings.HasPrefix(i, "/") {
		i = "/" + i
	}
	return i
}

// NewSecret is a random 24-character value safe in URLs and env files.
func NewSecret() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Choices are what the user set in the popup: env values by "service/KEY",
// published ports by "service/target/protocol".
type Choices struct {
	Values map[string]string `json:"values"`
	Ports  map[string]string `json:"ports"`
}

// PortKey names a port in Choices.
func PortKey(p Port) string { return p.Service + "/" + p.Target + "/" + p.Protocol }

// Apply writes the choices into the file. Unknown keys, bad ports and any
// placeholder left anywhere are errors; the file isn't installable with them.
func (d *Doc) Apply(c Choices) error {
	for k, v := range c.Values {
		svc, key, ok := strings.Cut(k, "/")
		if !ok || strings.ContainsAny(v, "\n\r") || !d.SetEnv(svc, key, v) {
			return fmt.Errorf("unknown setting %q", k)
		}
	}
	for k, v := range c.Ports {
		parts := strings.Split(k, "/")
		if len(parts) != 3 || !validPort(v) || !d.SetPublished(parts[0], parts[1], parts[2], v) {
			return fmt.Errorf("unknown port %q or bad number %q", k, v)
		}
	}
	var left []string
	for _, e := range d.Envs() {
		if HasPlaceholder(e.Value) {
			left = append(left, e.Key)
		}
	}
	if len(left) > 0 {
		sort.Strings(left)
		return &MissingError{Keys: left}
	}
	return nil
}

// MissingError lists settings that still hold a placeholder.
type MissingError struct{ Keys []string }

func (e *MissingError) Error() string {
	return "fill in " + strings.Join(e.Keys, ", ") + " first"
}

func validPort(v string) bool {
	if len(v) == 0 || len(v) > 5 {
		return false
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 1 && n <= 65535
}

// KeepFrom carries an installed copy's env values and published ports into
// this (newer) file, for an app update: what the user set stays.
func (d *Doc) KeepFrom(old *Doc) {
	have := map[string]string{}
	for _, e := range old.Envs() {
		have[e.Service+"/"+e.Key] = e.Value
	}
	for _, e := range d.Envs() {
		if v, ok := have[e.Service+"/"+e.Key]; ok {
			d.SetEnv(e.Service, e.Key, v)
		}
	}
	pub := map[string]string{}
	for _, p := range old.Ports() {
		pub[PortKey(p)] = p.Published
	}
	for _, p := range d.Ports() {
		if v, ok := pub[PortKey(p)]; ok && v != p.Published {
			d.SetPublished(p.Service, p.Target, p.Protocol, v)
		}
	}
}

// HostOnly strips a port from a Host header value.
func HostOnly(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(h, "[]")
	}
	return strings.Trim(hostport, "[]")
}
