// internal/version/version.go
package version

// Version is the release version of Argos Prob. It is embedded in every
// snapshot sent to the master (host.Snapshot.Version) and exposed on the
// active-mode /health endpoint. The master compares it with the stable
// Store release available for the agent platform and architecture.
// Version is a variable so release builds can override it with go build -ldflags -X.
var Version = "1.5.1"
