// Package compose reads and edits an app's docker-compose.yml as a YAML tree,
// so changing an env value or a port keeps everything else as it was.
//
// It knows the shapes IceStore and ZimaOS use (PLAN.md, Phase 2, "What is
// known already"): environment as a list ("KEY=value") or a map, ports in the
// long form (target/published) or the short "published:target", and the
// x-casaos blocks at the top and per service.
package compose

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Doc is one compose file.
type Doc struct {
	root *yaml.Node // the document node
}

// Parse reads a compose file. It must be a mapping with a services mapping.
func Parse(b []byte) (*Doc, error) {
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return nil, fmt.Errorf("compose file: %w", err)
	}
	if n.Kind != yaml.DocumentNode || len(n.Content) != 1 || n.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("compose file: not a mapping")
	}
	d := &Doc{root: &n}
	if s := get(d.top(), "services"); s == nil || s.Kind != yaml.MappingNode || len(s.Content) == 0 {
		return nil, errors.New("compose file: no services")
	}
	return d, nil
}

// Bytes writes the file back out.
func (d *Doc) Bytes() ([]byte, error) {
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if err := e.Encode(d.root); err != nil {
		return nil, err
	}
	_ = e.Close()
	return b.Bytes(), nil
}

func (d *Doc) top() *yaml.Node { return d.root.Content[0] }

// get returns the value of key in a mapping node, or nil.
func get(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func str(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	return n.Value
}

// Name is the project name (top-level "name"), which ZimaOS uses as the app's name.
func (d *Doc) Name() string { return str(get(d.top(), "name")) }

// X is the top-level x-casaos block, or nil.
func (d *Doc) X() *yaml.Node { return get(d.top(), "x-casaos") }

// XString reads a scalar from the top-level x-casaos block.
func (d *Doc) XString(key string) string { return str(get(d.X(), key)) }

type service struct {
	name string
	node *yaml.Node
}

func (d *Doc) services() []service {
	s := get(d.top(), "services")
	var out []service
	for i := 0; i+1 < len(s.Content); i += 2 {
		if s.Content[i+1].Kind == yaml.MappingNode {
			out = append(out, service{s.Content[i].Value, s.Content[i+1]})
		}
	}
	return out
}

func (d *Doc) service(name string) *yaml.Node {
	for _, s := range d.services() {
		if s.name == name {
			return s.node
		}
	}
	return nil
}

// Main is the app's main service: x-casaos main, else the first service.
func (d *Doc) Main() string {
	if m := d.XString("main"); m != "" && d.service(m) != nil {
		return m
	}
	return d.services()[0].name
}

// Container is the main service's container name (container_name, else the service name).
func (d *Doc) Container() string {
	m := d.Main()
	if c := str(get(d.service(m), "container_name")); c != "" {
		return c
	}
	return m
}

// Env is one environment value of one service.
type Env struct {
	Service     string `json:"service"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// Envs lists every service's environment in file order, with the
// per-service x-casaos descriptions where the file has them.
func (d *Doc) Envs() []Env {
	var out []Env
	for _, s := range d.services() {
		desc := envDescriptions(s.node)
		env := get(s.node, "environment")
		if env == nil {
			continue
		}
		switch env.Kind {
		case yaml.SequenceNode:
			for _, it := range env.Content {
				k, v, _ := strings.Cut(it.Value, "=")
				if k = strings.TrimSpace(k); k != "" {
					out = append(out, Env{s.name, k, v, desc[k]})
				}
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(env.Content); i += 2 {
				k := env.Content[i].Value
				out = append(out, Env{s.name, k, env.Content[i+1].Value, desc[k]})
			}
		}
	}
	return out
}

// envDescriptions reads x-casaos.envs (container → description.en_US) of one service.
func envDescriptions(svc *yaml.Node) map[string]string {
	out := map[string]string{}
	envs := get(get(svc, "x-casaos"), "envs")
	if envs == nil || envs.Kind != yaml.SequenceNode {
		return out
	}
	for _, e := range envs.Content {
		k := str(get(e, "container"))
		if k == "" {
			continue
		}
		dn := get(e, "description")
		if dn != nil && dn.Kind == yaml.MappingNode {
			out[k] = strings.TrimSpace(str(get(dn, "en_US")))
		} else {
			out[k] = strings.TrimSpace(str(dn))
		}
	}
	return out
}

// SetEnv changes an existing value. It reports whether the key was found.
func (d *Doc) SetEnv(svc, key, value string) bool {
	env := get(d.service(svc), "environment")
	if env == nil {
		return false
	}
	switch env.Kind {
	case yaml.SequenceNode:
		for _, it := range env.Content {
			if k, _, _ := strings.Cut(it.Value, "="); strings.TrimSpace(k) == key {
				it.Value = key + "=" + value
				it.Style = 0
				it.Tag = "!!str"
				return true
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(env.Content); i += 2 {
			if env.Content[i].Value == key {
				v := env.Content[i+1]
				v.Kind, v.Tag, v.Value, v.Style = yaml.ScalarNode, "!!str", value, 0
				return true
			}
		}
	}
	return false
}

// Port is one published port of one service.
type Port struct {
	Service     string `json:"service"`
	Target      string `json:"target"`
	Published   string `json:"published"`
	Protocol    string `json:"protocol"`
	Description string `json:"description,omitempty"`
}

// Ports lists the published ports.
func (d *Doc) Ports() []Port {
	var out []Port
	for _, s := range d.services() {
		desc := portDescriptions(s.node)
		ports := get(s.node, "ports")
		if ports == nil || ports.Kind != yaml.SequenceNode {
			continue
		}
		for _, p := range ports.Content {
			switch p.Kind {
			case yaml.MappingNode:
				t, pub, proto := str(get(p, "target")), str(get(p, "published")), str(get(p, "protocol"))
				if proto == "" {
					proto = "tcp"
				}
				if pub != "" {
					out = append(out, Port{s.name, t, pub, proto, desc[t]})
				}
			case yaml.ScalarNode:
				if pt, ok := parseShort(p.Value); ok {
					pt.Service, pt.Description = s.name, desc[pt.Target]
					out = append(out, pt)
				}
			}
		}
	}
	return out
}

// parseShort reads "published:target[/proto]" (an optional host IP in front is kept out).
func parseShort(v string) (Port, bool) {
	proto := "tcp"
	if a, b, ok := strings.Cut(v, "/"); ok {
		v, proto = a, b
	}
	parts := strings.Split(v, ":")
	if len(parts) < 2 {
		return Port{}, false
	}
	return Port{Published: parts[len(parts)-2], Target: parts[len(parts)-1], Protocol: proto}, true
}

func portDescriptions(svc *yaml.Node) map[string]string {
	out := map[string]string{}
	ports := get(get(svc, "x-casaos"), "ports")
	if ports == nil || ports.Kind != yaml.SequenceNode {
		return out
	}
	for _, e := range ports.Content {
		if k := str(get(e, "container")); k != "" {
			out[k] = strings.TrimSpace(str(get(get(e, "description"), "en_US")))
		}
	}
	return out
}

// SetPublished moves a published port. When it was the app's port_map, that moves too.
func (d *Doc) SetPublished(svc, target, protocol, published string) bool {
	ports := get(d.service(svc), "ports")
	if ports == nil || ports.Kind != yaml.SequenceNode {
		return false
	}
	for _, p := range ports.Content {
		var old string
		switch p.Kind {
		case yaml.MappingNode:
			proto := str(get(p, "protocol"))
			if proto == "" {
				proto = "tcp"
			}
			if str(get(p, "target")) != target || proto != protocol {
				continue
			}
			n := get(p, "published")
			if n == nil {
				continue
			}
			old = n.Value
			n.Kind, n.Tag, n.Value, n.Style = yaml.ScalarNode, "!!str", published, yaml.DoubleQuotedStyle
		case yaml.ScalarNode:
			pt, ok := parseShort(p.Value)
			if !ok || pt.Target != target || pt.Protocol != protocol {
				continue
			}
			old = pt.Published
			p.Value = strings.Replace(p.Value, old+":"+target, published+":"+target, 1)
		default:
			continue
		}
		if pm := get(d.X(), "port_map"); pm != nil && pm.Value == old {
			pm.Value = published
		}
		return true
	}
	return false
}

// Access is what the app asks of the server beyond its own container.
type Access struct {
	Privileged   bool `json:"privileged"`
	DockerSocket bool `json:"docker_socket"`
	HostNetwork  bool `json:"host_network"`
}

// Access reads privileged, a mounted Docker socket and host networking from every service.
func (d *Doc) Access() Access {
	var a Access
	for _, s := range d.services() {
		if str(get(s.node, "privileged")) == "true" {
			a.Privileged = true
		}
		if str(get(s.node, "network_mode")) == "host" {
			a.HostNetwork = true
		}
		vols := get(s.node, "volumes")
		if vols == nil {
			continue
		}
		for _, v := range vols.Content {
			src := v.Value
			if v.Kind == yaml.MappingNode {
				src = str(get(v, "source"))
			}
			if strings.Contains(src, "docker.sock") {
				a.DockerSocket = true
			}
		}
	}
	return a
}
