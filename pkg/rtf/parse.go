package rtf

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	charmap "golang.org/x/text/encoding/charmap"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// group is the state of a group of an RTF document: its destination, which
// is empty for the document's text, whether its text is skipped, and how
// many characters follow a Unicode character, which are skipped
type group struct {
	destination string
	skip        bool
	uc          int
}

// parser reads an RTF document, a group at a time
type parser struct {
	data    []byte
	pos     int
	stack   []group
	charmap *charmap.Charmap

	// The text of the destination being read, the paragraph of the
	// document's text, the date of a time in the info group, and how many
	// characters are still to be skipped after a Unicode character
	text      strings.Builder
	paragraph strings.Builder
	date      [5]int
	pending   int

	// The paragraphs of the document's text, and the properties in its info
	// group, by their control words, such as title, its times, such as
	// creatim, and its counts, such as nofpages
	paragraphs []string
	properties map[string]string
	times      map[string]time.Time
	counts     map[string]int
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// skipped are the destinations whose text isn't the document's, such as its
// tables of fonts and colours, and its pictures, headers and footers
var skipped = map[string]bool{
	"fonttbl": true, "colortbl": true, "stylesheet": true, "listtable": true,
	"listoverridetable": true, "revtbl": true, "rsidtbl": true, "filetbl": true,
	"pict": true, "object": true, "objdata": true, "themedata": true,
	"colorschememapping": true, "latentstyles": true, "datastore": true,
	"xmlnstbl": true, "fldinst": true, "header": true, "headerl": true,
	"headerr": true, "headerf": true, "footer": true, "footerl": true,
	"footerr": true, "footerf": true, "pgdsctbl": true, "mmathPr": true,
	"bkmkstart": true, "bkmkend": true, "nonshppict": true, "shppict": true,
}

// properties are the destinations in the info group which are text
var properties = map[string]bool{
	"title": true, "subject": true, "author": true, "operator": true,
	"keywords": true, "doccomm": true, "company": true, "manager": true,
	"category": true,
}

// times are the destinations in the info group which are times, made of the
// control words yr, mo, dy, hr and min
var times = map[string]bool{"creatim": true, "revtim": true, "printim": true}

// dateFields are the control words of a time in the info group, by the
// index of their field of the date: its year, month, day, hour and minute
var dateFields = map[string]int{"yr": 0, "mo": 1, "dy": 2, "hr": 3, "min": 4}

// counts are the control words in the info group which are counts
var counts = map[string]bool{"nofpages": true, "nofwords": true, "nofchars": true}

// symbols are the control words which are characters
var symbols = map[string]string{
	"tab": "\t", "line": "\n", "emdash": "—", "endash": "–", "emspace": " ",
	"enspace": " ", "qmspace": " ", "bullet": "•", "lquote": "‘",
	"rquote": "’", "ldblquote": "“", "rdblquote": "”",
	"cell": "\t", "row": "\n", "sect": "\n", "page": "\n",
}

// charmaps are the character sets of the code pages of documents, by their
// numbers, such as 1252, which is Windows-1252, and is a document's when it
// doesn't have one, or its code page isn't here
var charmaps = map[int]*charmap.Charmap{
	437: charmap.CodePage437, 850: charmap.CodePage850, 852: charmap.CodePage852,
	866: charmap.CodePage866, 1250: charmap.Windows1250, 1251: charmap.Windows1251,
	1252: charmap.Windows1252, 1253: charmap.Windows1253, 1254: charmap.Windows1254,
	1255: charmap.Windows1255, 1256: charmap.Windows1256, 1257: charmap.Windows1257,
	1258: charmap.Windows1258, 10000: charmap.Macintosh, 10007: charmap.MacintoshCyrillic,
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Parse returns the RTF document: the text of its paragraphs, decoded from
// its code page, such as Windows-1252 or Mac Roman, without the text of its
// font and style tables, pictures, headers, footers and fields'
// instructions, and the properties in its info group. A document is read up
// to its end, or the end of its outermost group, and what isn't RTF is read
// as text.
func Parse(data []byte) *Document {
	p := &parser{
		data:       data,
		charmap:    charmap.Windows1252,
		stack:      []group{{uc: 1}},
		properties: make(map[string]string),
		times:      make(map[string]time.Time),
		counts:     make(map[string]int),
	}
	p.parse()
	return p.document()
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// parse reads the document, up to its end, or the end of its outermost group
func (p *parser) parse() {
	for p.pos < len(p.data) {
		switch c := p.data[p.pos]; c {
		case '{':
			p.pos++
			p.push()
		case '}':
			p.pos++
			if !p.pop() {
				p.endParagraph()
				return
			}
		case '\\':
			p.pos++
			p.control()
		case '\r', '\n':
			p.pos++
		default:
			p.pos++
			p.byte(c)
		}
	}
	p.endParagraph()
}

// document returns the document's paragraphs, and the properties in its info
// group
func (p *parser) document() *Document {
	return &Document{
		Paragraphs: p.paragraphs,
		Info: Info{
			Title:      p.properties["title"],
			Subject:    p.properties["subject"],
			Author:     p.properties["author"],
			Operator:   p.properties["operator"],
			Keywords:   p.properties["keywords"],
			Comment:    p.properties["doccomm"],
			Company:    p.properties["company"],
			Manager:    p.properties["manager"],
			Category:   p.properties["category"],
			Generator:  p.properties["generator"],
			Created:    p.times["creatim"],
			Revised:    p.times["revtim"],
			Printed:    p.times["printim"],
			Pages:      p.counts["nofpages"],
			Words:      p.counts["nofwords"],
			Characters: p.counts["nofchars"],
		},
	}
}

// push starts a group, which has its parent's state, after adding the text
// written to the paragraph, so a group whose text is skipped doesn't
// discard it
func (p *parser) push() {
	p.flush()
	p.stack = append(p.stack, p.top())
}

// pop ends a group, and its destination, when it has one, and returns false
// when it's the outermost group
func (p *parser) pop() bool {
	if len(p.stack) <= 1 {
		return false
	}
	g := p.top()
	p.stack = p.stack[:len(p.stack)-1]
	if g.destination != p.top().destination {
		p.endDestination(g.destination)
	}
	return len(p.stack) > 1
}

// top returns the state of the current group
func (p *parser) top() group {
	return p.stack[len(p.stack)-1]
}

// set sets the state of the current group
func (p *parser) set(g group) {
	p.stack[len(p.stack)-1] = g
}

// control reads a control word, such as \par or \u8217, or a control
// symbol, such as \' with two hex digits, or \{
func (p *parser) control() {
	if p.pos >= len(p.data) {
		return
	}
	c := p.data[p.pos]
	switch {
	case c == '\'':
		if p.pos+3 <= len(p.data) {
			if b, err := strconv.ParseUint(string(p.data[p.pos+1:p.pos+3]), 16, 8); err == nil {
				p.pos += 3
				p.byte(byte(b))
				return
			}
		}
		p.pos++
	case c == '*':
		// The group is a destination which is skipped, when it isn't known
		p.pos++
		g := p.top()
		g.skip = true
		p.set(g)
	case c == '\\' || c == '{' || c == '}':
		p.pos++
		p.byte(c)
	case c == '~':
		p.pos++
		p.write(" ")
	case c == '_':
		p.pos++
		p.write("-")
	case c == '\r' || c == '\n':
		p.pos++
		p.word("par", 0, false)
	case isLetter(c):
		start := p.pos
		for p.pos < len(p.data) && isLetter(p.data[p.pos]) {
			p.pos++
		}
		word := string(p.data[start:p.pos])
		numStart := p.pos
		if p.pos < len(p.data) && p.data[p.pos] == '-' {
			p.pos++
		}
		for p.pos < len(p.data) && p.data[p.pos] >= '0' && p.data[p.pos] <= '9' {
			p.pos++
		}
		n, err := strconv.Atoi(string(p.data[numStart:p.pos]))
		hasParam := err == nil
		if p.pos < len(p.data) && p.data[p.pos] == ' ' {
			p.pos++
		}
		p.word(word, n, hasParam)
	default:
		// Another control symbol, such as \- or \|, which isn't text
		p.pos++
	}
}

// word acts on a control word, and its parameter
func (p *parser) word(word string, n int, hasParam bool) {
	g := p.top()
	switch {
	case word == "ansicpg":
		if cm, exists := charmaps[n]; exists {
			p.charmap = cm
		}
	case word == "mac":
		p.charmap = charmap.Macintosh
	case word == "uc" && hasParam:
		g.uc = max(0, n)
		p.set(g)
	case word == "u" && hasParam:
		if n < 0 {
			n += 65536
		}
		p.writeRune(rune(n))
		p.pending = g.uc
	case word == "info":
		g.destination, g.skip = "info", false
		p.set(g)
	case g.destination == "info" && (properties[word] || times[word]):
		p.flush()
		g.destination = "info:" + word
		p.set(g)
		p.date = [5]int{}
	case strings.HasPrefix(g.destination, "info:") && times[strings.TrimPrefix(g.destination, "info:")]:
		if i, exists := dateFields[word]; exists && hasParam {
			p.date[i] = n
		}
	case g.destination == "info" && counts[word] && hasParam:
		p.counts[word] = n
	case skipped[word]:
		g.destination, g.skip = word, true
		p.set(g)
	case word == "generator":
		g.destination, g.skip = "generator", false
		p.set(g)
	case word == "cocoartf":
		p.properties["generator"] = "Cocoa"
	case word == "fldrslt":
		g.skip = false
		p.set(g)
	case word == "par":
		p.endParagraph()
	default:
		if symbol, exists := symbols[word]; exists {
			p.write(symbol)
		}
	}
}

// byte writes a character of the document's code page
func (p *parser) byte(c byte) {
	if p.pending > 0 {
		p.pending--
		return
	}
	p.writeRune(p.charmap.DecodeByte(c))
}

// writeRune writes a character, which is skipped in a group whose text is
// skipped
func (p *parser) writeRune(r rune) {
	if p.top().skip || r == utf8.RuneError {
		return
	}
	p.text.WriteRune(r)
}

// write writes text, as writeRune does
func (p *parser) write(s string) {
	if p.pending > 0 {
		p.pending = 0
	}
	if !p.top().skip {
		p.text.WriteString(s)
	}
}

// flush adds the text written to the paragraph, when it's the document's
// text, and discards it otherwise
func (p *parser) flush() {
	if p.top().destination == "" {
		p.paragraph.WriteString(p.text.String())
	}
	p.text.Reset()
}

// endParagraph ends a paragraph of the document's text, which is added when
// it isn't blank
func (p *parser) endParagraph() {
	if p.top().destination != "" {
		return
	}
	p.flush()
	if text := strings.TrimSpace(p.paragraph.String()); text != "" {
		p.paragraphs = append(p.paragraphs, text)
	}
	p.paragraph.Reset()
}

// endDestination ends a destination, whose text, or time, is a property of
// the document, when it's in its info group
func (p *parser) endDestination(destination string) {
	name, isInfo := strings.CutPrefix(destination, "info:")
	switch {
	case destination == "generator":
		if text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p.text.String()), ";")); text != "" {
			p.properties["generator"] = text
		}
		p.text.Reset()
	case !isInfo:
		if destination != "" {
			p.text.Reset()
		}
	case times[name]:
		if d := p.date; d[0] > 0 && d[1] >= 1 && d[1] <= 12 && d[2] >= 1 {
			p.times[name] = time.Date(d[0], time.Month(d[1]), d[2], d[3], d[4], 0, 0, time.UTC)
		}
		p.text.Reset()
	default:
		if text := strings.Join(strings.Fields(p.text.String()), " "); text != "" {
			p.properties[name] = text
		}
		p.text.Reset()
	}
}

// isLetter returns true when the character is an ASCII letter, as control
// words are
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
