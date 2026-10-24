package dublincore

import "testing"

func Test_types_000(t *testing.T) {
	// Every kind has a DCMI type, and something which isn't a kind has none
	for _, kind := range []string{KindArchive, KindAudio, KindBook, KindCode, KindData, KindDocument, KindForm, KindImage, KindPresentation, KindSoftware, KindSpreadsheet, KindVideo, KindWebPage} {
		if TypeOf(kind) == "" {
			t.Errorf("%s: expected a type", kind)
		}
	}
	if got := TypeOf("book"); got != TypeText {
		t.Errorf("expected Text, got %q", got)
	}
	if got := TypeOf("unknown"); got != "" {
		t.Errorf("expected no type, got %q", got)
	}
}
