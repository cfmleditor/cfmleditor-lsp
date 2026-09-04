package path

import (
	"testing"

	"go.lsp.dev/uri"
)

// TestToURIWindowsMatchesIssue45 pins the exact URI reported in
// cfmleditor/cfmleditor-lsp#45: window/showDocument was handed
// "file://C:\\Users\\quetw\\...", which a Windows client cannot open.
func TestToURIWindowsMatchesIssue45(t *testing.T) {
	const (
		in   = `C:\Users\quetw\IdeaProjects\TestProject\src\Application.cfc`
		want = "file:///c%3A/Users/quetw/IdeaProjects/TestProject/src/Application.cfc"
	)

	if got := ToURIFor(uri.PlatformWindows, in); string(got) != want {
		t.Errorf("ToURIFor(windows, %q)\n got  %s\n want %s", in, got, want)
	}

	// What the concatenation produced, kept here so the regression is legible.
	if bad := "file://" + in; bad == want {
		t.Fatal("the broken form and the correct form cannot be equal")
	}
}

func TestToURIEncodesAndPreserves(t *testing.T) {
	cases := map[string]string{
		"/app/Application.cfc":              "file:///app/Application.cfc",
		"/app/My Documents/Application.cfc": "file:///app/My%20Documents/Application.cfc",
		"/app/a+b/Application.cfc":          "file:///app/a%2Bb/Application.cfc",
		"/app/100%25/Application.cfc":       "file:///app/100%2525/Application.cfc",
	}

	for in, want := range cases {
		if got := ToURIFor(uri.PlatformPOSIX, in); string(got) != want {
			t.Errorf("ToURIFor(posix, %q) = %s, want %s", in, got, want)
		}
	}
}

// TestFromURIAcceptsEveryShapeAClientSends is the tolerance requirement. A
// canonical Windows URI has to decode to a usable path, and the raw form some
// clients send has to keep working, because switching to the URI library's own
// accessor alone returns "" for it: the whole path parses as the authority.
func TestFromURIAcceptsEveryShapeAClientSends(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"canonical windows", `file:///c%3A/Users/q/App.cfc`, `c:\Users\q\App.cfc`},
		{"windows unencoded colon", `file:///C:/Users/q/App.cfc`, `c:\Users\q\App.cfc`},
		{"windows raw backslashes", `file://C:\Users\q\App.cfc`, `C:\Users\q\App.cfc`},
		{"already a path", `C:\Users\q\App.cfc`, `C:\Users\q\App.cfc`},
		{"not a file uri", `untitled:Untitled-1`, `untitled:Untitled-1`},
	}

	for _, c := range cases {
		if got := FromURIFor(uri.PlatformWindows, c.in); got != c.want {
			t.Errorf("%s: FromURIFor(windows, %q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestFromURIPOSIX(t *testing.T) {
	cases := map[string]string{
		"file:///app/Application.cfc":        "/app/Application.cfc",
		"file:///app/My%20Documents/App.cfc": "/app/My Documents/App.cfc",
		"file:///app/My Documents/App.cfc":   "/app/My Documents/App.cfc",
		"/app/Application.cfc":               "/app/Application.cfc",
	}

	for in, want := range cases {
		if got := FromURIFor(uri.PlatformPOSIX, in); got != want {
			t.Errorf("FromURIFor(posix, %q) = %q, want %q", in, got, want)
		}
	}
}

// TestRoundTrip is the property that matters at the boundary: a path handed to
// a client and read back has to survive.
func TestRoundTrip(t *testing.T) {
	for _, platform := range []uri.Platform{uri.PlatformPOSIX, uri.PlatformWindows} {
		paths := []string{"/app/Application.cfc", "/app/My Documents/App.cfc"}
		if platform == uri.PlatformWindows {
			paths = []string{`c:\Users\q\App.cfc`, `c:\Users\q\My Documents\App.cfc`}
		}

		for _, p := range paths {
			if got := FromURIFor(platform, string(ToURIFor(platform, p))); got != p {
				t.Errorf("platform %d: round trip of %q gave %q", platform, p, got)
			}
		}
	}
}
