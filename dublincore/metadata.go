package dublincore

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	Namespace = "dc"
)

// Names of the fifteen elements of the Dublin Core Metadata Element Set,
// version 1.1, which are keys in the dc namespace, such as
// meta.New(Namespace, Title, value)
// (see https://www.dublincore.org/specifications/dublin-core/dces/)
const (
	Contributor = "contributor" // An entity responsible for making contributions to the resource
	Coverage    = "coverage"    // The spatial or temporal topic of the resource, or its jurisdiction
	Creator     = "creator"     // An entity primarily responsible for making the resource
	Date        = "date"        // A point or period of time associated with the resource
	Description = "description" // An account of the resource
	Format      = "format"      // The file format, physical medium, or dimensions of the resource
	Identifier  = "identifier"  // An unambiguous reference to the resource
	Language    = "language"    // A language of the resource
	Publisher   = "publisher"   // An entity responsible for making the resource available
	Relation    = "relation"    // A related resource
	Rights      = "rights"      // Information about rights held in and over the resource
	Source      = "source"      // A related resource from which the resource is derived
	Subject     = "subject"     // The topic of the resource
	Title       = "title"       // A name given to the resource
	Type        = "type"        // The nature or genre of the resource
)
