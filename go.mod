module github.com/HSinghHira/IceDash

go 1.27.0

require (
	github.com/chicohaager/lintux-modkit v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

// lintux-modkit isn't published as a Go module: `make modkit` checks it out
// into .modkit at the commit named in lintux-modkit.ref.
// Stay on v0.1.3: from v0.1.4 its auth.Verifier lets only the "admin" role
// in, which would lock out ZimaOS "user" accounts (read-only in IceDash) and
// every CasaOS sign-in (no role claim). Bump only once IceDash checks roles
// itself, or modkit can let other roles through (heads-up from Lintux,
// 2026-11-16).
replace github.com/chicohaager/lintux-modkit => ./.modkit
