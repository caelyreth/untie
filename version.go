package untie

const (
	libraryVersion = "0.1.0"
	layoutRevision = "1"
	version        = libraryVersion + "+layout." + layoutRevision
)

// Version returns the library and layout revision used in cache contracts.
func Version() string {
	return version
}
