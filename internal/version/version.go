// internal/version/version.go
package version

// Version is the release version of Argos Prob. It is embedded in every
// snapshot sent to the master (host.Snapshot.Version) and exposed on the
// active-mode /health endpoint. The master warns when an agent is older
// than its expected LATEST_AGENT_VERSION.
const Version = "1.1.0"
