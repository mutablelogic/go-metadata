package title

import "testing"

func Test_title_000(t *testing.T) {
	// A title which is empty, the name of a file, or an application's
	// default title isn't a title
	for title, want := range map[string]bool{
		"":                               true,
		"  ":                             true,
		"Microsoft Word - Document1.doc": true,
		"Microsoft Excel - Book1.xls":    true,
		"PowerPoint Presentation":        true,
		"Annual Report":                  false,
		"Notes about Microsoft Word":     false,
	} {
		if got := Untitled(title); got != want {
			t.Errorf("%q: expected %v, got %v", title, want, got)
		}
	}
}
