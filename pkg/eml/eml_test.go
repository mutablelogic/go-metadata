package eml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Packages
	eml "github.com/mutablelogic/go-metadata/pkg/eml"
)

// TEST_DIR is the folder of the test files
const TEST_DIR = "../../testfiles"

// lines returns the lines joined with CRLF
func lines(lines ...string) string {
	return strings.Join(lines, "\r\n") + "\r\n"
}

// parse returns the message, or fails the test
func parse(t *testing.T, data string) *eml.Message {
	t.Helper()
	m, err := eml.Parse(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// parseFile returns the message in the test file, or fails the test
func parseFile(t *testing.T, name string) *eml.Message {
	t.Helper()
	f, err := os.Open(filepath.Join(TEST_DIR, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m, err := eml.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// names returns the names of the attachments
func names(attachments []eml.Attachment) []string {
	var result []string
	for _, a := range attachments {
		result = append(result, a.Name)
	}
	return result
}

func Test_eml_000(t *testing.T) {
	// The subject, addresses, date and identifier, and the text
	m := parse(t, lines(
		"From: Ada Lovelace <ada@example.com>",
		"To: Charles Babbage <charles@example.com>, mary@example.com",
		"Cc: <lord@example.com>",
		"Subject: The Analytical Engine",
		"Date: Tue, 21 Oct 2026 09:30:00 +0200",
		"Message-ID: <abc123@example.com>",
		"",
		"Dear Charles,",
		"",
		"Here are my notes.",
	))
	if m.Subject != "The Analytical Engine" || m.MessageID != "abc123@example.com" {
		t.Errorf("subject %q, identifier %q", m.Subject, m.MessageID)
	}
	if len(m.From) != 1 || m.From[0].Name != "Ada Lovelace" || m.From[0].Address != "ada@example.com" {
		t.Errorf("from %v", m.From)
	}
	if len(m.To) != 2 || m.To[1].Address != "mary@example.com" || len(m.Cc) != 1 || m.Cc[0].Address != "lord@example.com" {
		t.Errorf("to %v, cc %v", m.To, m.Cc)
	}
	if want := time.Date(2026, 10, 21, 7, 30, 0, 0, time.UTC); !m.Date.Equal(want) {
		t.Errorf("date %v, want %v", m.Date, want)
	}
	if want := "Dear Charles,\n\nHere are my notes.\n"; m.Text != want {
		t.Errorf("text %q, want %q", m.Text, want)
	}
}

func Test_eml_001(t *testing.T) {
	// An encoded subject and sender, the plain text and HTML of the
	// alternatives, decoded from quoted-printable, and an attachment, decoded
	// from base64
	m := parse(t, lines(
		"From: =?UTF-8?Q?Caf=C3=A9?= <cafe@example.com>",
		"Subject: =?UTF-8?B?TWVudSBmb3IgY2Fmw6k=?=",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		`Content-Type: multipart/alternative; boundary="inner"`,
		"",
		"--inner",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Soup of the day: caf=C3=A9 au lait",
		"--inner",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>Soup</p>",
		"--inner--",
		"--outer",
		`Content-Type: application/pdf; name="menu.pdf"`,
		`Content-Disposition: attachment; filename="menu.pdf"`,
		"Content-Transfer-Encoding: base64",
		"",
		"JVBERi0x",
		"LjQK",
		"--outer--",
	))
	if m.Subject != "Menu for café" || m.From[0].Name != "Café" {
		t.Errorf("subject %q, from %v", m.Subject, m.From)
	}
	if m.Text != "Soup of the day: café au lait" || m.HTML != "<p>Soup</p>" {
		t.Errorf("text %q, HTML %q", m.Text, m.HTML)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "menu.pdf" || m.Attachments[0].ContentType != "application/pdf" || string(m.Attachments[0].Data) != "%PDF-1.4\n" {
		t.Errorf("attachments %+v", m.Attachments)
	}
}

func Test_eml_002(t *testing.T) {
	// A text attachment without a name isn't the text, which follows it
	m := parse(t, lines(
		"Subject: Notes",
		`Content-Type: multipart/mixed; boundary="b"`,
		"",
		"--b",
		"Content-Type: text/plain",
		"Content-Disposition: attachment",
		"",
		"An attachment",
		"--b",
		"Content-Type: text/plain",
		"",
		"The body",
		"--b--",
	))
	if m.Text != "The body" {
		t.Errorf("text %q", m.Text)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "" || string(m.Attachments[0].Data) != "An attachment" {
		t.Errorf("attachments %+v", m.Attachments)
	}
}

func Test_eml_003(t *testing.T) {
	// Text in base64, broken into lines, and in Latin-1, and Windows-1252's
	// quotes, dashes and euro sign, which Latin-1 is read as, in the
	// subject and the text, and text which is labelled ASCII, but is UTF-8
	if m := parse(t, lines("Content-Type: text/plain; charset=utf-8", "Content-Transfer-Encoding: base64", "", "SGVsbG8s", "IHdvcmxk")); m.Text != "Hello, world" {
		t.Errorf("base64 %q", m.Text)
	}
	if m := parse(t, "Content-Type: text/plain; charset=iso-8859-1\r\n\r\ncaf\xe9\r\n"); m.Text != "café\n" {
		t.Errorf("Latin-1 %q", m.Text)
	}
	for _, charset := range []string{"windows-1252", "iso-8859-1"} {
		m := parse(t, "Subject: =?"+charset+"?Q?=93Price=94_=96_=8020?=\r\nContent-Type: text/plain; charset="+charset+"\r\n\r\nIt\x92s \x805\x85\r\n")
		if m.Subject != "“Price” – €20" || m.Text != "It’s €5…\n" {
			t.Errorf("%s: subject %q, text %q", charset, m.Subject, m.Text)
		}
	}
	if m := parse(t, "Content-Type: text/plain; charset=us-ascii\r\n\r\ncafé\r\n"); m.Text != "café\n" {
		t.Errorf("ASCII %q", m.Text)
	}

	// A subject which isn't encoded, but is in Windows-1252, as an old mail
	// client sent it
	if m := parse(t, "Subject: R\xe9sum\xe9\r\n\r\nHi\r\n"); m.Subject != "Résumé" {
		t.Errorf("subject %q", m.Subject)
	}
}

func Test_eml_004(t *testing.T) {
	// Attachments which Eudora saved to a folder, on Windows and a Mac
	m := parse(t, lines(
		`X-Attachments: C:\EUDORA\attach\report.doc; Macintosh HD:Eudora Folder:Attachments Folder:photo.jpg;`,
		"Subject: Report",
		"",
		"Attached.",
	))
	if got := names(m.Attachments); strings.Join(got, ",") != "report.doc,photo.jpg" {
		t.Errorf("attachments %q", got)
	}
	if m.Attachments[0].ContentType == "" || m.Attachments[0].Data != nil {
		t.Errorf("attachment %+v", m.Attachments[0])
	}
}

func Test_eml_005(t *testing.T) {
	// A message without a header, such as one which Eudora saved without
	// it, is all text, whether it starts with a blank line, which is the end
	// of an empty header, or with text
	m := parseFile(t, "eml_eudora.eml")
	if m.Subject != "" || len(m.From) != 0 || len(m.Header) != 0 || !strings.HasPrefix(m.Text, "this sounds like an excellent idea...\n") {
		t.Errorf("subject %q, from %v, header %v, text %q", m.Subject, m.From, m.Header, m.Text[:min(60, len(m.Text))])
	}
	if m := parse(t, "Please phone Alex about the talk.\r\nThanks\r\n"); m.Text != "Please phone Alex about the talk.\nThanks\n" {
		t.Errorf("text %q", m.Text)
	}
}

func Test_eml_006(t *testing.T) {
	// A message from Outlook Express in Big5, whose recipient's name is an
	// encoded word, whose alternatives are empty, and which has an HTML
	// attachment
	m := parseFile(t, "eml_outlook.eml")
	if m.Subject != "new product catalog (Netbeeper)" || m.From[0].Name != "Example Trading" {
		t.Errorf("subject %q, from %v", m.Subject, m.From)
	}
	if len(m.To) != 4 || m.To[0].Name != "kay 陳小姐" || m.To[3].Address != "gary@example.net" {
		t.Errorf("to %v", m.To)
	}
	if want := time.Date(2001, 4, 27, 1, 44, 25, 0, time.UTC); !m.Date.Equal(want) {
		t.Errorf("date %v, want %v", m.Date, want)
	}
	if strings.TrimSpace(m.Text) != "" || !strings.Contains(m.HTML, "<BODY bgColor=#ffffff>") {
		t.Errorf("text %q, HTML %q", m.Text, m.HTML)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].Name != "I7B02 HTML.htm" || !strings.Contains(string(m.Attachments[0].Data), "Lets people know who") {
		t.Errorf("attachments %q", names(m.Attachments))
	}
}
