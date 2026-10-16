package brand

import (
	"encoding/base64"
	"encoding/xml"
	"strings"
	"testing"
)

// TestLogoIsASelfContainedSVGDataURI pins the contract the host relies on: the
// management centre puts this string into an <img src=…>, so it has to be a
// single-line data URI whose payload is the embedded mark itself.
func TestLogoIsASelfContainedSVGDataURI(t *testing.T) {
	const prefix = "data:image/svg+xml;base64,"
	if !strings.HasPrefix(Logo, prefix) {
		t.Fatalf("Logo does not start with %q: %.60s", prefix, Logo)
	}
	if strings.ContainsAny(Logo, "\n\r\"'&<>") {
		t.Fatalf("Logo contains characters that need escaping: %q", Logo)
	}
	encoded := strings.TrimPrefix(Logo, prefix)
	decoded, errDecode := base64.StdEncoding.DecodeString(encoded)
	if errDecode != nil {
		t.Fatalf("Logo payload is not valid base64: %v", errDecode)
	}
	if string(decoded) != Markup {
		t.Fatalf("Logo payload differs from Markup:\n got %s\nwant %s", decoded, Markup)
	}
}

// TestMarkupIsAParseableSquareSVG keeps the embedded asset honest: the
// dashboard inlines it as-is, so it has to be well-formed XML with an svg root
// and the official 512x512 square mark.
func TestMarkupIsAParseableSquareSVG(t *testing.T) {
	if strings.TrimSpace(Markup) != Markup {
		t.Fatal("Markup has surrounding whitespace and cannot be inlined as-is")
	}
	decoder := xml.NewDecoder(strings.NewReader(Markup))
	root := ""
	depth := 0
	elements := 0
	for {
		token, errToken := decoder.Token()
		if errToken != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				root = value.Name.Local
			}
			depth++
			elements++
		case xml.EndElement:
			depth--
		}
	}
	if root != "svg" {
		t.Fatalf("Markup root element = %q, want svg", root)
	}
	if depth != 0 {
		t.Fatalf("Markup is not balanced (depth %d)", depth)
	}
	if elements < 3 {
		t.Fatalf("Markup has %d elements, want the background and the two mark paths", elements)
	}
	for _, want := range []string{`width="512"`, `height="512"`, `viewBox="0 0 512 512"`, "#131010", "#5A5858", "white"} {
		if !strings.Contains(Markup, want) {
			t.Errorf("Markup is missing %q", want)
		}
	}
}

// TestIdentityIsUsable guards the strings the management centre and the
// dashboard display.
func TestIdentityIsUsable(t *testing.T) {
	if strings.TrimSpace(Name) == "" {
		t.Error("Name is empty")
	}
	if strings.TrimSpace(Author) == "" {
		t.Error("Author is empty")
	}
	if !strings.HasPrefix(Repository, "https://github.com/") {
		t.Errorf("Repository = %q, want a GitHub URL", Repository)
	}
	if !strings.Contains(Attribution, "opencode.ai") {
		t.Errorf("Attribution does not credit the upstream project: %q", Attribution)
	}
}
