module github.com/cpa-opencodego/cap-opencode-go

go 1.26.0

require (
	github.com/router-for-me/CLIProxyAPI/v8 v8.0.27
	go.etcd.io/bbolt v1.4.3
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/sys v0.47.0 // indirect

// Build against the pinned local checkout of CLIProxyAPI v8.0.27.
// Only public sdk/pluginapi and sdk/pluginabi are imported, so no third-party
// module downloads are required for the build.
replace github.com/router-for-me/CLIProxyAPI/v8 => ../ref/cpa/CLIProxyAPI-v8
