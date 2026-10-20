// Package brand holds the OpenCode Go display identity: the mark every protocol
// face shows in the CLIProxyAPI plugin management page and in the plugin's own
// dashboard, plus the product name and repository the four faces share.
//
// One product, four faces: the provider is split into four shared libraries so
// that CLIProxyAPI performs every client-protocol translation (see
// docs/design.md). The management centre lists one row per library, so the four
// rows have to read as four faces of the same plugin rather than four separate
// plugins — this package is the single source for what they all display.
package brand

import (
	_ "embed"
	"encoding/base64"
	"strings"
)

// embeddedMarkup is the OpenCode Go mark: the official OpenCode square icon
// from https://opencode.ai/favicon.svg, with the redundant wrapper and the
// unused prefers-color-scheme <style> block dropped. The same file is published
// as assets/logo.svg for the repository, README and plugin store listing.
//
// OpenCode is a project of Anomaly (https://github.com/anomalyco/opencode,
// MIT). The mark is used here descriptively, to identify the upstream service
// this plugin talks to; it is not covered by this repository's license.
//
//go:embed opencode.svg
var embeddedMarkup string

// Markup is the mark without surrounding whitespace, so it can be inlined into
// the dashboard document exactly as embedded.
var Markup = strings.TrimSpace(embeddedMarkup)

// Logo is the value handed to the host as plugin metadata. The management
// centre renders it both in the plugin row and as the icon of the plugin's
// sidebar entry, and resolves the field through an allow-list that accepts
// http(s), data: and blob: sources. A data: URI keeps the icon working without
// a network round trip and without shipping a separate asset route.
var Logo = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(Markup))

// Product identity shared by every protocol face.
const (
	// Name is the product name. The main face registers it as-is; the three
	// sibling faces append their role label, so the four management rows read
	// as one product with four protocol faces.
	Name = "OpenCode Go"
	// Author identifies the plugin author or organization.
	Author = "cpa-opencodego"
	// Repository is the repository URL for plugin source and support. The
	// management centre renders it as the plugin row's "open repository" link
	// and the dashboard links it from the about panel.
	Repository = "https://github.com/Pheobe-Southwood/cpa-opencodego"
	// Attribution credits the upstream project whose mark is embedded above.
	Attribution = "OpenCode icon © 2025 opencode (https://opencode.ai, MIT)"
)
