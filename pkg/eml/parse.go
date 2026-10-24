package eml

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"unicode/utf8"

	// Packages
	charmap "golang.org/x/text/encoding/charmap"
	htmlindex "golang.org/x/text/encoding/htmlindex"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// header is the header of a message, or of a part of one
type header interface {
	Get(key string) string
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// maxDepth is the most parts within parts which are read
const maxDepth = 5

// wordDecoder decodes the encoded words of a header, such as
// =?big5?B?...?=, in any character set which a browser reads
var wordDecoder = &mime.WordDecoder{CharsetReader: charsetReader}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns the email message, which is read to its end: its subject,
// addresses, date and identifier, its first plain text and HTML, and its
// attachments, including those which Eudora saved to a folder, and listed in
// its X-Attachments header. A message whose header can't be read, such as one
// which Eudora saved without it, is all text.
func Parse(r io.Reader) (*Message, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return &Message{Header: mail.Header{}, Text: decodeText("", data)}, nil
	}

	h := msg.Header
	m := &Message{
		Header:    h,
		Subject:   decodeHeader(h.Get("Subject")),
		From:      addresses(h, "From"),
		To:        addresses(h, "To"),
		Cc:        addresses(h, "Cc"),
		MessageID: strings.Trim(strings.TrimSpace(h.Get("Message-Id")), "<>"),
	}
	if date, err := h.Date(); err == nil {
		m.Date = date
	}
	m.readPart(h, msg.Body, 0)
	m.Attachments = append(m.Attachments, eudoraAttachments(h.Get("X-Attachments"))...)
	return m, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// readPart reads a part of the message, with its header, and the parts
// within it, to maxDepth: its first plain text and HTML part which isn't an
// attachment is its text and HTML, and any other part is an attachment
func (m *Message) readPart(h header, r io.Reader, depth int) {
	contentType, params, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		contentType, params = "text/plain", map[string]string{}
	}
	disposition, dparams, _ := mime.ParseMediaType(h.Get("Content-Disposition"))
	name := decodeHeader(firstOf(dparams["filename"], params["name"]))
	attachment := strings.EqualFold(disposition, "attachment")

	switch {
	case strings.HasPrefix(contentType, "multipart/"):
		if depth >= maxDepth {
			return
		}
		reader := multipart.NewReader(r, params["boundary"])
		for {
			part, err := reader.NextRawPart()
			if err != nil {
				return
			}
			m.readPart(part.Header, part, depth+1)
		}
	case contentType == "text/plain" && !attachment && m.Text == "":
		if data, err := io.ReadAll(decodeTransfer(h.Get("Content-Transfer-Encoding"), r)); err == nil {
			m.Text = decodeText(params["charset"], data)
		}
	case contentType == "text/html" && !attachment && m.HTML == "":
		if data, err := io.ReadAll(decodeTransfer(h.Get("Content-Transfer-Encoding"), r)); err == nil {
			m.HTML = decodeText(params["charset"], data)
		}
	default:
		data, _ := io.ReadAll(decodeTransfer(h.Get("Content-Transfer-Encoding"), r))
		m.Attachments = append(m.Attachments, Attachment{Name: name, ContentType: contentType, Data: data})
	}
}

// addresses returns the addresses of the header, whose names have their
// encoded words decoded, or none when it has none. An address which can't be
// read is left out.
func addresses(h mail.Header, key string) []*mail.Address {
	value := h.Get(key)
	if value == "" {
		return nil
	}
	parser := mail.AddressParser{WordDecoder: wordDecoder}
	list, err := parser.ParseList(value)
	if err != nil {
		// Read the addresses one at a time, as one which can't be read
		// stops the others being read
		list = nil
		for _, part := range strings.Split(value, ",") {
			if address, err := parser.Parse(part); err == nil {
				list = append(list, address)
			}
		}
	}
	for _, address := range list {
		address.Name = decodeHeader(address.Name)
	}
	return list
}

// eudoraAttachments returns the attachments which Eudora saved to a folder,
// from its X-Attachments header, which is their paths, each followed by a
// semicolon, such as C:\EUDORA\attach\report.doc; on Windows, or
// Macintosh HD:Eudora Folder:Attachments Folder:report.doc; on a Mac
func eudoraAttachments(value string) []Attachment {
	var result []Attachment
	for _, path := range strings.Split(value, ";") {
		path = strings.TrimSpace(decodeHeader(path))
		if i := strings.LastIndexAny(path, `\/:`); i >= 0 {
			path = path[i+1:]
		}
		if path != "" {
			result = append(result, Attachment{Name: path, ContentType: mime.TypeByExtension(pathExt(path))})
		}
	}
	return result
}

// decodeHeader returns a header's value, with its encoded words decoded, and
// as Windows-1252 when it isn't UTF-8, as a header which isn't ASCII is
// usually in a sender's Windows code page
func decodeHeader(value string) string {
	if decoded, err := wordDecoder.DecodeHeader(value); err == nil {
		value = decoded
	}
	if !utf8.ValidString(value) {
		value = decodeText("windows-1252", []byte(value))
	}
	return strings.TrimSpace(controls(value))
}

// controls returns the text with Latin-1's control characters, from 0x80 to
// 0x9F, as Windows-1252's characters, as the word decoder decodes Latin-1
// itself, and they're usually Windows-1252's quotes, dashes and euro sign
func controls(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 0x80 && r <= 0x9F {
			if c := charmap.Windows1252.DecodeByte(byte(r)); c != utf8.RuneError {
				return c
			}
		}
		return r
	}, text)
}

// decodeTransfer returns a reader of a part's content, decoded from its
// transfer encoding
func decodeTransfer(encoding string, r io.Reader) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	default:
		return r
	}
}

// decodeText returns text in the character set as UTF-8, with its line
// breaks as newlines. Text which is ASCII, or is labelled ASCII but is UTF-8,
// is as it is. A character set which a browser reads is decoded as a browser
// does, so Latin-1 is Windows-1252, whose bytes 0x80 to 0x9F are quotes,
// dashes and the euro sign, and text in another character set, or none, is
// UTF-8 when it's valid, or else Windows-1252.
func decodeText(charset string, data []byte) string {
	charset = strings.ToLower(strings.TrimSpace(charset))
	text, decoded := "", false
	switch {
	case isASCII(data), (charset == "us-ascii" || charset == "ascii") && utf8.Valid(data):
		text, decoded = string(data), true
	case charset != "":
		if enc, err := htmlindex.Get(charset); err == nil {
			if b, err := enc.NewDecoder().Bytes(data); err == nil {
				text, decoded = string(b), true
			}
		}
	}
	if !decoded {
		if utf8.Valid(data) {
			text = string(data)
		} else {
			b, _ := charmap.Windows1252.NewDecoder().Bytes(data)
			text = string(b)
		}
	}
	return strings.ReplaceAll(text, "\r\n", "\n")
}

// charsetReader returns a reader of text in the character set as UTF-8, for
// decoding a header's encoded words, or an error for a character set which
// a browser doesn't read
func charsetReader(charset string, r io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return nil, err
	}
	return enc.NewDecoder().Reader(r), nil
}

// isASCII returns true when the data is ASCII
func isASCII(data []byte) bool {
	for _, b := range data {
		if b >= 0x80 {
			return false
		}
	}
	return true
}

// pathExt returns the extension of a file's name, such as .doc
func pathExt(name string) string {
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		return strings.ToLower(name[i:])
	}
	return ""
}

// firstOf returns the first value which isn't empty
func firstOf(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
