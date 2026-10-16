// Package dashboard renders the plugin page embedded in the CLIProxyAPI
// management centre.
//
// The page is served from an unauthenticated resource route, so it never
// renders secrets. Everything it displays comes from the plugin's own
// credential-free resource routes (/api/models, /api/quota), which is what lets
// the whole page stay editable without a password: the operator can switch
// models, edit aliases and change settings, and the host management credential
// is only needed at the moment a change is written. When the management centre
// stored one (the operator ticked 记住密码 on its login page) it is reused from
// the centre's own storage; otherwise the page asks for it once, in a dialog,
// and the plugin itself still keeps no credential anywhere.
//
// Layout notes: the plugin page is rendered in an iframe that the management
// centre places under its own fixed header, therefore
//   - the page fills the iframe with a flex column: a top action bar, a tab
//     strip, and one scrollable region. There is deliberately no bottom bar: on
//     phones the management centre can leave a strip of its own background under
//     the iframe, and a fixed bottom bar would float above that strip;
//   - the header height is measured from the parent document and applied as a
//     top inset, so the host's floating header never covers plugin content.
package dashboard

import "strings"

// Page returns the dashboard document for a plugin id and role.
func Page(pluginID, role string) []byte {
	document := strings.ReplaceAll(pageTemplate, "__PLUGIN_ID__", jsString(pluginID))
	document = strings.ReplaceAll(document, "__ROLE__", jsString(role))
	return []byte(document)
}

// jsString escapes a value for embedding inside a JavaScript string literal.
func jsString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "<", `\u003c`, ">", `\u003e`, "&", `\u0026`)
	return replacer.Replace(value)
}

// pageTemplate is assembled from four parts so that each concern can be edited
// on its own: pageHead (document head), pageStyles (the stylesheet), pageMarkup
// (the body) and pageScript (the single script).
const pageTemplate = pageHead + pageStyles + pageHeadEnd + pageMarkup + "<script>" + pageScript + "</script>" + pageFoot
