package version

// manually set constant version
const version string = "v11"

// Get returns the Go module version of the axiom-go module.
func Get() string {
	return version
}
