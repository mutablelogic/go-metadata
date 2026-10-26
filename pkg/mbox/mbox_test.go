package mbox_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	// Packages
	mbox "github.com/mutablelogic/go-metadata/pkg/mbox"
)

// messages returns the messages of the mailbox, or fails the test
func messages(t *testing.T, data string) []*mbox.Message {
	t.Helper()
	var result []*mbox.Message
	r := mbox.NewReader(strings.NewReader(data))
	for {
		m, err := r.Next()
		if errors.Is(err, io.EOF) {
			return result
		} else if err != nil {
			t.Fatal(err)
		}
		result = append(result, m)
	}
}

func Test_mbox_000(t *testing.T) {
	// Messages after From lines, with their senders and dates, where a line
	// which starts with From, without a date, is a line of a message, and
	// one escaped with > is unescaped, with any line endings, and without a
	// blank line before a From line, as Eudora writes
	got := messages(t, "From ada@example.com Mon Feb 11 13:12:43 2002\r\n"+
		"Subject: One\r\n\r\nFrom the start\r\n>From here\r\n>>From there\r\n"+
		"From ???@??? Tue Feb 12 10:02:33 2002\r"+
		"Subject: Two\r\rHi\r"+
		"From charles@example.com Wed Feb 13 09:30:00 +0100 2002 remote from engine\n"+
		"Subject: Three\n\nBye\n")
	if len(got) != 3 {
		t.Fatalf("got %d messages", len(got))
	}
	want := []struct {
		sender string
		date   time.Time
		data   string
	}{
		{"ada@example.com", time.Date(2002, 2, 11, 13, 12, 43, 0, time.UTC), "Subject: One\n\nFrom the start\nFrom here\n>From there\n"},
		{"", time.Date(2002, 2, 12, 10, 2, 33, 0, time.UTC), "Subject: Two\n\nHi\n"},
		{"charles@example.com", time.Date(2002, 2, 13, 8, 30, 0, 0, time.UTC), "Subject: Three\n\nBye\n"},
	}
	for i, w := range want {
		if got[i].Sender != w.sender || !got[i].Date.Equal(w.date) || string(got[i].Data) != w.data {
			t.Errorf("%d: got %q, %v, %q", i, got[i].Sender, got[i].Date, got[i].Data)
		}
	}
}

func Test_mbox_001(t *testing.T) {
	// Text before the first From line isn't a message, and a mailbox which
	// is empty, or has no From lines, has none
	if got := messages(t, "\r\nnot a message\r\nFrom a@example.com Mon Feb 11 13:12:43 2002\r\n\r\nHi\r\n"); len(got) != 1 || string(got[0].Data) != "\nHi\n" {
		t.Errorf("got %d messages", len(got))
	}
	for _, data := range []string{"", "Subject: Not a mailbox\n\nFrom the start\n"} {
		if got := messages(t, data); len(got) != 0 {
			t.Errorf("%q: got %d messages", data, len(got))
		}
	}
}

func Test_mbox_002(t *testing.T) {
	// A message which is longer than MaxMessageSize is cut short, and the
	// next message is read
	long := strings.Repeat("x", 1<<20) + "\n"
	got := messages(t, "From a@example.com Mon Feb 11 13:12:43 2002\n"+strings.Repeat(long, mbox.MaxMessageSize/len(long)+2)+
		"From b@example.com Tue Feb 12 10:02:33 2002\nSubject: Next\n")
	if len(got) != 2 || len(got[0].Data) > mbox.MaxMessageSize+1 || string(got[1].Data) != "Subject: Next\n" {
		t.Errorf("got %d messages, of %d bytes", len(got), len(got[0].Data))
	}
}

func Test_mbox_writer_000(t *testing.T) {
	// Messages which are written are read as they were written, with their
	// senders and dates, in UTC, to the second, and lines which start with
	// From, or which are escaped already, as they were, including an empty
	// message, between messages, and at the end, and a message which is a
	// blank line
	sent := []*mbox.Message{
		{Sender: "ada@example.com", Date: time.Date(2026, 10, 26, 9, 15, 30, 0, time.UTC), Data: []byte("Subject: One\n\nFrom the start\n>From here\n>>From there\n")},
		{Sender: "", Date: time.Date(2026, 10, 26, 10, 0, 0, 0, time.FixedZone("CET", 3600)), Data: []byte("Subject: Two\n\nEnds with a blank line\n\n")},
		{Sender: "charles@example.com", Date: time.Date(2002, 2, 13, 8, 30, 0, 0, time.UTC), Data: []byte("Subject: Three\n\nBye\n")},
		{Sender: "empty@example.com", Date: time.Date(2002, 2, 14, 8, 30, 0, 0, time.UTC), Data: []byte("")},
		{Sender: "blank@example.com", Date: time.Date(2002, 2, 15, 8, 30, 0, 0, time.UTC), Data: []byte("\n")},
		{Sender: "last@example.com", Date: time.Date(2002, 2, 16, 8, 30, 0, 0, time.UTC), Data: []byte("")},
	}
	var b strings.Builder
	w := mbox.NewWriter(&b)
	for _, m := range sent {
		if err := w.Write(m); err != nil {
			t.Fatal(err)
		}
	}
	got := messages(t, b.String())
	if len(got) != len(sent) {
		t.Fatalf("got %d messages, want %d:\n%s", len(got), len(sent), b.String())
	}
	for i, m := range sent {
		if got[i].Sender != m.Sender || !got[i].Date.Equal(m.Date) || string(got[i].Data) != string(m.Data) {
			t.Errorf("%d: got %q, %v, %q", i, got[i].Sender, got[i].Date, got[i].Data)
		}
	}
}

func Test_mbox_writer_001(t *testing.T) {
	// A From line has the sender, as one word, or ???@??? without one, and
	// the date in UTC, and the message's lines end with line feeds, whatever
	// they ended with, and with one at its end, and a blank line after it
	var b strings.Builder
	w := mbox.NewWriter(&b)
	date := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := w.Write(&mbox.Message{Sender: "Ada Lovelace", Date: date, Data: []byte("Subject: Hi\r\n\r\nFrom me\rBye")}); err != nil {
		t.Fatal(err)
	}
	if err := w.Write(&mbox.Message{Date: date}); err != nil {
		t.Fatal(err)
	}
	want := "From Ada_Lovelace Fri Jan  2 03:04:05 2026\nSubject: Hi\n\n>From me\nBye\n\n" +
		"From ???@??? Fri Jan  2 03:04:05 2026\n\n"
	if b.String() != want {
		t.Errorf("got %q, want %q", b.String(), want)
	}
}
