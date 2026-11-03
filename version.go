package untie

const (
	libraryVersion = "0.4.0"
	layoutRevision = "6"
	version        = libraryVersion + "+layout." + layoutRevision
)

// Version returns the library and layout revision used in cache contracts.
func Version() string {
	return version
}
