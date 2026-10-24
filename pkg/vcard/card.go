// Package vcard defines a vCard, a contact, such as a person or an
// organisation, which is encoded as vCard 4.0, and decoded from vCard 2.1,
// 3.0 or 4.0, and its photo, which can be embedded in it, as artwork.
// Reference: https://www.rfc-editor.org/rfc/rfc6350
package vcard

import (
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Card is a contact, such as a person or an organisation
type Card struct {
	UID        string         `json:"uid,omitempty"`        // Its unique identifier
	Kind       string         `json:"kind,omitempty"`       // individual, group, org or location, or empty for an individual
	FN         string         `json:"fn,omitempty"`         // Its formatted name, such as Ada Lovelace
	Name       Name           `json:"name,omitzero"`        // The parts of its name
	Nicknames  []string       `json:"nicknames,omitempty"`  // Its nicknames
	Emails     []Value        `json:"emails,omitempty"`     // Its email addresses
	Phones     []Value        `json:"phones,omitempty"`     // Its phone numbers
	URLs       []Value        `json:"urls,omitempty"`       // Its URLs
	Addresses  []Address      `json:"addresses,omitempty"`  // Its addresses
	Orgs       []Organization `json:"orgs,omitempty"`       // The organisations it's part of
	Titles     []string       `json:"titles,omitempty"`     // Its job titles
	Birthday   *Date          `json:"birthday,omitempty"`   // Its birthday
	Notes      []string       `json:"notes,omitempty"`      // Notes about it
	Categories []string       `json:"categories,omitempty"` // Its categories, or tags
	Photo      *Photo         `json:"photo,omitempty"`      // Its photo, embedded or linked
	Rev        time.Time      `json:"rev,omitzero"`         // When it was last changed
}

// Name is the parts of a name, any of which can be empty
type Name struct {
	Family string `json:"family,omitempty"` // Lovelace
	Given  string `json:"given,omitempty"`  // Ada
	Middle string `json:"middle,omitempty"` // Augusta
	Prefix string `json:"prefix,omitempty"` // Countess
	Suffix string `json:"suffix,omitempty"` // FRS
}

// Value is a value, such as an email address or phone number, with its
// types, such as home or work, in lower case, and whether it's preferred
type Value struct {
	Value string   `json:"value"`
	Types []string `json:"types,omitempty"`
	Pref  bool     `json:"pref,omitempty"`
}

// Address is a postal address, with its types, such as home or work, in
// lower case
type Address struct {
	Types      []string `json:"types,omitempty"`
	POBox      string   `json:"po_box,omitempty"`
	Extended   string   `json:"extended,omitempty"` // Such as a flat or suite
	Street     string   `json:"street,omitempty"`
	Locality   string   `json:"locality,omitempty"` // Such as a city
	Region     string   `json:"region,omitempty"`   // Such as a state or county
	PostalCode string   `json:"postal_code,omitempty"`
	Country    string   `json:"country,omitempty"`
}

// Organization is an organisation, and the units within it, such as a
// department
type Organization struct {
	Name  string   `json:"name,omitempty"`
	Units []string `json:"units,omitempty"`
}

// Date is a date, such as a birthday, whose year is zero when it isn't
// known
type Date struct {
	Year  int `json:"year,omitempty"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// DisplayName returns the name of the contact, which is its formatted name,
// or else the parts of its name, or its first email address, or an empty
// string when it has none of them
func (c *Card) DisplayName() string {
	if name := strings.TrimSpace(c.FN); name != "" {
		return name
	} else if name := c.Name.String(); name != "" {
		return name
	}
	for _, email := range c.Emails {
		if value := strings.TrimSpace(email.Value); value != "" {
			return value
		}
	}
	return ""
}

// String returns the parts of a name, in the order they're said, such as
// Countess Ada Augusta Lovelace FRS
func (n Name) String() string {
	return join(" ", n.Prefix, n.Given, n.Middle, n.Family, n.Suffix)
}

// String returns the parts of an address which aren't empty, separated with
// commas, such as 1 Main St, London, UK
func (a Address) String() string {
	return join(", ", a.POBox, a.Extended, a.Street, a.Locality, a.Region, a.PostalCode, a.Country)
}

// String returns an organisation and its units, separated with commas
func (o Organization) String() string {
	return join(", ", append([]string{o.Name}, o.Units...)...)
}

// IsZero returns true when the name has no parts
func (n Name) IsZero() bool {
	return n == Name{}
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// join returns the values which aren't empty, joined with the separator
func join(separator string, values ...string) string {
	var result []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return strings.Join(result, separator)
}
