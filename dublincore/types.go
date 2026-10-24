package dublincore

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The terms of the DCMI Type Vocabulary, which are the values of the type
// element, dc:type, the nature or genre of a resource
// (see https://www.dublincore.org/specifications/dublin-core/dcmi-type-vocabulary/)
const (
	TypeCollection          = "Collection"          // An aggregation of resources, such as an archive
	TypeDataset             = "Dataset"             // Data encoded in a defined structure, such as a spreadsheet
	TypeEvent               = "Event"               // A non-persistent, time-based occurrence
	TypeImage               = "Image"               // A visual representation other than text
	TypeInteractiveResource = "InteractiveResource" // A resource requiring interaction from the user, such as a form
	TypeMovingImage         = "MovingImage"         // A series of visual representations, such as a video
	TypePhysicalObject      = "PhysicalObject"      // An inanimate, three-dimensional object or substance
	TypeService             = "Service"             // A system that provides one or more functions
	TypeSoftware            = "Software"            // A computer program in source or compiled form
	TypeSound               = "Sound"               // A resource primarily intended to be heard
	TypeStillImage          = "StillImage"          // A static visual representation, such as a photo
	TypeText                = "Text"                // A resource consisting primarily of words for reading
)

// The kinds of files, which are finer than the DCMI types, such as a book
// rather than text, as metadata:kind
const (
	KindArchive      = "archive"      // An archive of files, such as a zip file
	KindAudio        = "audio"        // Audio, such as music or a podcast
	KindBook         = "book"         // A book, such as an EPUB or Kindle book
	KindCalendar     = "calendar"     // A calendar of events or tasks
	KindCode         = "code"         // Source code
	KindContact      = "contact"      // A contact, such as a person in an address book
	KindData         = "data"         // Structured data, such as JSON or YAML
	KindDocument     = "document"     // A document, such as a PDF or Word document, or plain text
	KindEpisode      = "episode"      // An episode of a TV show
	KindFilm         = "film"         // A film, such as a movie
	KindForm         = "form"         // A form, which is filled in
	KindImage        = "image"        // An image, such as a photo or a drawing
	KindPresentation = "presentation" // A presentation of slides
	KindSoftware     = "software"     // Software, such as an application or a library
	KindSpreadsheet  = "spreadsheet"  // A spreadsheet
	KindSubtitles    = "subtitles"    // Subtitles of a video, such as a SubRip file
	KindVideo        = "video"        // A video
	KindWebPage      = "webpage"      // A web page
)

// kindTypes are the DCMI types of the kinds
var kindTypes = map[string]string{
	KindArchive:      TypeCollection,
	KindAudio:        TypeSound,
	KindBook:         TypeText,
	KindCalendar:     TypeEvent,
	KindCode:         TypeSoftware,
	KindContact:      TypeText,
	KindData:         TypeDataset,
	KindDocument:     TypeText,
	KindEpisode:      TypeMovingImage,
	KindFilm:         TypeMovingImage,
	KindForm:         TypeInteractiveResource,
	KindImage:        TypeStillImage,
	KindPresentation: TypeText,
	KindSoftware:     TypeSoftware,
	KindSpreadsheet:  TypeDataset,
	KindSubtitles:    TypeText,
	KindVideo:        TypeMovingImage,
	KindWebPage:      TypeText,
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// TypeOf returns the DCMI type of the kind of a file, such as Text for a
// book, or an empty string when it's not a kind
func TypeOf(kind string) string {
	return kindTypes[kind]
}
