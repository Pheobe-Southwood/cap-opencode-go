// Package dashboard tests assert the properties the operator cares about after
// the v1.2.0 rework, so a later edit cannot silently bring back a password
// prompt or a bottom-anchored bar.
package dashboard

import (
	"strings"
	"testing"
)

func page(t *testing.T) string {
	t.Helper()
	return string(Page("cap-opencode-go", "chat"))
}

func TestPageSubstitutesPlaceholders(t *testing.T) {
	document := page(t)
	if strings.Contains(document, "__PLUGIN_ID__") || strings.Contains(document, "__ROLE__") {
		t.Fatal("placeholders were not substituted")
	}
	if !strings.Contains(document, `var PLUGIN_ID = "cap-opencode-go"`) {
		t.Error("plugin id missing from the document")
	}
	if !strings.Contains(document, `var ROLE = "chat"`) {
		t.Error("role missing from the document")
	}
}

// The plugin must never ask the operator for a password: the credential comes
// from the management centre's own storage and nothing else.
func TestPageHasNoPasswordPrompt(t *testing.T) {
	document := page(t)
	for _, forbidden := range []string{"window.prompt", "sessionStorage", "请输入", "管理密钥（用于保存", "forceKey"} {
		if strings.Contains(document, forbidden) {
			t.Errorf("the page must not contain %q", forbidden)
		}
	}
	if !strings.Contains(document, "cli-proxy-auth") {
		t.Error("the page must read the management centre credential store")
	}
	if !strings.Contains(document, "'managementKey'") {
		t.Error("the page must keep the legacy credential fallback")
	}
}

// The page fills the iframe with a top action bar and one scrolling region.
// Nothing may be anchored to the bottom edge: on phones the management centre
// can leave a strip of its own background below the iframe.
func TestPageHasNoBottomAnchoredBar(t *testing.T) {
	document := page(t)
	if strings.Contains(document, "position:fixed") {
		t.Error("no element may be fixed-positioned; the page must not float over the parent's strip")
	}
	if !strings.Contains(document, "class=\"toolbar\"") {
		t.Error("the top action bar is missing")
	}
	if !strings.Contains(document, "--host-header") {
		t.Error("the host header inset variable is missing")
	}
	if !strings.Contains(document, "flex-direction:column;height:100%") {
		t.Error("the page must be a full-height flex column")
	}
}

// Model settings are collapsed behind a per-row button, and the search box is
// part of the model card so it is visible without scrolling past three cards.
func TestPageModelControls(t *testing.T) {
	document := page(t)
	for _, want := range []string{
		`id="modelFilter"`,
		`data-model-settings="`,
		`data-model-alias="`,
		`data-model-family="`,
		`data-model-proxy="`,
		`data-model-probe="`,
		"opencode-go/probe?model=",
		"收起全部设置",
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the page is missing %q", want)
		}
	}
	// Settings inputs are only rendered by the expandable panel.
	if !strings.Contains(document, "if(editable && isOpen)") {
		t.Error("the settings panel must be rendered only when expanded")
	}
}

func TestJSStringEscapesInjection(t *testing.T) {
	document := string(Page(`</SCRIPT><script>alert(1)</script>`, "chat"))
	if strings.Contains(document, "<script>alert(1)</script>") {
		t.Fatal("plugin ids must not be able to close the script tag")
	}
	if !strings.Contains(document, `\u003c/SCRIPT\u003e`) {
		t.Error("angle brackets must be escaped")
	}
}

// A save replaces the whole plugin configuration block, so the page must never
// write values it has not read, and must refuse to save at all when the block
// could not be read.
func TestPageGuardsAgainstOverwritingUnreadConfiguration(t *testing.T) {
	document := page(t)
	for _, want := range []string{
		"var configLoaded = false;",
		"if(!configLoaded){",
		"已取消保存以免覆盖",
		"function resolved(id, fallback)",
		"renderAdvanced();",
	} {
		if !strings.Contains(document, want) {
			t.Errorf("the page is missing the guard %q", want)
		}
	}
	// The resolved view must be written back into the working copy.
	if !strings.Contains(document, "config['base-url'] = fields.cfgBaseURL;") {
		t.Error("renderAdvanced must persist the resolved values into config")
	}
}
