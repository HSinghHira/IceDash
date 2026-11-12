module github.com/HSinghHira/IceDash

go 1.27.0

require (
	github.com/chicohaager/lintux-modkit v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

// lintux-modkit isn't published as a Go module: `make modkit` checks it out
// into .modkit at the commit named in lintux-modkit.ref.
replace github.com/chicohaager/lintux-modkit => ./.modkit
