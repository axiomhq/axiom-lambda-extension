package version

// manually set constant version
const version string = "v3"

// Get returns the Go module version of the axiom-go module.
func Get() string {
	return version
}
