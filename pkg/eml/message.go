// Package eml reads email messages, in the Internet Message Format, with
// MIME parts, such as .eml files, and the messages in a mailbox: their
// subject, addresses, date and identifier, their text and HTML, decoded from
// their transfer encoding and character set, and their attachments
package eml

import (
	"net/mail"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Message is an email message
type Message struct {
	// The message's header, as it is, which is empty for a message without
	// one, such as a message which Eudora saved without its header
	Header mail.Header

	// The subject, with its encoded words decoded, such as =?big5?B?...?=
	Subject string

	// The addresses of the senders, the recipients, and the recipients who
	// are copied, whose names have their encoded words decoded
	From, To, Cc []*mail.Address

	// When the message was sent, which is zero when it isn't known
	Date time.Time

	// The message's identifier, without its angle brackets
	MessageID string

	// The message's first plain text, and its first HTML, decoded from
	// their transfer encoding and character set, which are empty when it
	// has none. A message without a header is all text.
	Text, HTML string

	// The message's attachments, in their order
	Attachments []Attachment
}

// Attachment is an attachment of an email message
type Attachment struct {
	// Its file name, which is empty when it has none
	Name string

	// Its content type, such as application/pdf, which is empty when it
	// isn't known
	ContentType string

	// Its content, decoded from its transfer encoding, which is nil for an
	// attachment which isn't in the message, such as one which Eudora
	// saved to a folder, and listed in the message's X-Attachments header
	Data []byte
}

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ContentType is the content type of an email message
const ContentType = "message/rfc822"
