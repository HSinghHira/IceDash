package compose

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Images lists each service's image, by service name.
func (d *Doc) Images() map[string]string {
	out := map[string]string{}
	for _, s := range d.services() {
		if img := str(get(s.node, "image")); img != "" {
			out[s.name] = img
		}
	}
	return out
}

// Services lists the service names in file order.
func (d *Doc) Services() []string {
	var out []string
	for _, s := range d.services() {
		out = append(out, s.name)
	}
	return out
}

// SetImage points a service at another image. It reports whether the
// service has an image to change.
func (d *Doc) SetImage(svc, ref string) bool {
	n := get(d.service(svc), "image")
	if n == nil || n.Kind != yaml.ScalarNode {
		return false
	}
	n.Tag, n.Value, n.Style = "!!str", ref, 0
	return true
}

// Repo is an image reference without its tag or digest, with Docker Hub's
// default parts left out ("docker.io/library/nginx:1" → "nginx").
func Repo(ref string) string {
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	ref = strings.TrimPrefix(ref, "docker.io/")
	ref = strings.TrimPrefix(ref, "index.docker.io/")
	return strings.TrimPrefix(ref, "library/")
}

// Pinned reports whether ref names an image by digest.
func Pinned(ref string) bool { return strings.Contains(ref, "@sha256:") }

// DigestRef picks, from an image's RepoDigests, the one for image's own
// repository ("" when none matches). Any of them names the same image, but
// the matching one keeps the registry the app came from.
func DigestRef(image string, repoDigests []string) string {
	want := Repo(image)
	for _, rd := range repoDigests {
		if Repo(rd) == want && Pinned(rd) {
			return rd
		}
	}
	return ""
}

// KeepChanges carries into d (an older version's file) the settings the user
// changed after an update: env values, published ports and folders that
// differ between after (the file right after the update) and now (the file
// as it is installed now). Values the newer version brought itself stay out.
func (d *Doc) KeepChanges(after, now *Doc) {
	was := map[string]string{}
	for _, e := range after.Envs() {
		was[e.Service+"/"+e.Key] = e.Value
	}
	// Changed values, and keys the user added after the update.
	for _, e := range now.Envs() {
		k := e.Service + "/" + e.Key
		if w, inAfter := was[k]; inAfter && w == e.Value {
			continue
		}
		if !d.SetEnv(e.Service, e.Key, e.Value) {
			d.AddEnv(e.Service, e.Key, e.Value)
		}
	}

	wasPort := map[string]string{}
	for _, p := range after.Ports() {
		wasPort[PortKey(p)] = p.Published
	}
	for _, p := range now.Ports() {
		if w, ok := wasPort[PortKey(p)]; ok && w == p.Published {
			continue
		}
		if !d.SetPublished(p.Service, p.Target, p.Protocol, p.Published) {
			d.AddPort(p)
		}
	}

	wasVol := map[string]string{}
	for _, v := range after.Volumes() {
		wasVol[VolumeKey(v)] = v.Source
	}
	for _, v := range now.Volumes() {
		w, ok := wasVol[VolumeKey(v)]
		switch {
		case ok && w == v.Source:
		case d.service(v.Service) == nil:
		case d.HasTarget(v.Service, v.Target):
			d.SetSource(v.Service, v.Target, v.Source)
		default:
			d.AddBind(v.Service, v.Source, v.Target)
		}
	}
}

// AddEnv adds a value to a service's environment, in the form it already
// uses (a list when it has none). It reports whether the service exists.
func (d *Doc) AddEnv(svc, key, value string) bool {
	s := d.service(svc)
	if s == nil {
		return false
	}
	scalar := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
	env := get(s, "environment")
	if env == nil {
		s.Content = append(s.Content, scalar("environment"), &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		env = s.Content[len(s.Content)-1]
	}
	switch env.Kind {
	case yaml.SequenceNode:
		env.Content = append(env.Content, scalar(key+"="+value))
	case yaml.MappingNode:
		env.Content = append(env.Content, scalar(key), scalar(value))
	default:
		return false
	}
	return true
}

// AddPort publishes another port of a service, in the long form.
func (d *Doc) AddPort(p Port) bool {
	s := d.service(p.Service)
	if s == nil || p.Target == "" || p.Published == "" {
		return false
	}
	scalar := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
	ports := get(s, "ports")
	if ports == nil {
		s.Content = append(s.Content, scalar("ports"), &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"})
		ports = s.Content[len(s.Content)-1]
	}
	if ports.Kind != yaml.SequenceNode {
		return false
	}
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	pub := scalar(p.Published)
	pub.Style = yaml.DoubleQuotedStyle
	ports.Content = append(ports.Content, &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("target"), scalar(p.Target),
		scalar("published"), pub,
		scalar("protocol"), scalar(proto),
	}})
	return true
}
