// Package message registers the handlers of messages, such as email
// messages, and of mailboxes, so importing it anonymously registers them
// all
package message

import (
	// Handlers
	_ "github.com/mutablelogic/go-metadata/message/email"
	_ "github.com/mutablelogic/go-metadata/message/mbox"
)
