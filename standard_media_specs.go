package capns

// GetStandardMediaSpecs returns the built-in standard media specs
// These match the specs bundled in capns/standard/media/
//
// This provides explicit, fail-hard resolution for standard media URNs.
// If a media URN is not in this list, ResolveMediaUrn will fail.
func GetStandardMediaSpecs() []MediaSpecDef {
	return []MediaSpecDef{
		// Core primitive types
		{
			Urn:         "media:bytes",
			MediaType:   "application/octet-stream",
			Title:       "Bytes",
			ProfileURI:  "https://capns.org/schema/bytes",
			Description: "Raw byte sequence.",
		},
		{
			Urn:         "media:textable;form=scalar",
			MediaType:   "text/plain",
			Title:       "String",
			ProfileURI:  "https://capns.org/schema/string",
			Description: "UTF-8 string value.",
		},
		{
			Urn:         "media:form=map;textable",
			MediaType:   "application/json",
			Title:       "Map",
			ProfileURI:  "https://capns.org/schema/map",
			Description: "String-map map value.",
		},
		{
			Urn:         "media:form=list;textable",
			MediaType:   "application/json",
			Title:       "List",
			ProfileURI:  "https://capns.org/schema/list",
			Description: "Array/list value.",
		},
		{
			Urn:         "media:textable;numeric;form=scalar",
			MediaType:   "text/plain",
			Title:       "Number",
			ProfileURI:  "https://capns.org/schema/number",
			Description: "Numeric scalar value.",
		},
		{
			Urn:         "media:bool;textable;form=scalar",
			MediaType:   "text/plain",
			Title:       "Boolean",
			ProfileURI:  "https://capns.org/schema/boolean",
			Description: "Boolean value.",
		},
		{
			Urn:         "media:integer;textable;numeric;form=scalar",
			MediaType:   "text/plain",
			Title:       "Integer",
			ProfileURI:  "https://capns.org/schema/integer",
			Description: "Integer value.",
		},
		{
			Urn:         "media:void",
			MediaType:   "application/octet-stream",
			Title:       "Void",
			ProfileURI:  "https://capns.org/schema/void",
			Description: "No input/output.",
		},
		// Document types
		{
			Urn:         "media:pdf;bytes",
			MediaType:   "application/pdf",
			Title:       "PDF",
			ProfileURI:  "https://capns.org/schema/pdf",
			Description: "PDF document.",
			Extensions:  []string{"pdf"},
		},
		{
			Urn:         "media:epub;bytes",
			MediaType:   "application/epub+zip",
			Title:       "EPUB",
			ProfileURI:  "https://capns.org/schema/epub",
			Description: "EPUB document.",
			Extensions:  []string{"epub"},
		},
		// Text format types
		{
			Urn:         "media:md;textable",
			MediaType:   "text/markdown",
			Title:       "Markdown",
			ProfileURI:  "https://capns.org/schema/md",
			Description: "Markdown text.",
			Extensions:  []string{"md", "markdown"},
		},
		{
			Urn:         "media:txt;textable",
			MediaType:   "text/plain",
			Title:       "Plain Text",
			ProfileURI:  "https://capns.org/schema/txt",
			Description: "Plain text.",
			Extensions:  []string{"txt"},
		},
		{
			Urn:         "media:html;textable",
			MediaType:   "text/html",
			Title:       "HTML",
			ProfileURI:  "https://capns.org/schema/html",
			Description: "HTML document.",
			Extensions:  []string{"html", "htm"},
		},
		{
			Urn:         "media:xml;textable",
			MediaType:   "text/xml",
			Title:       "XML",
			ProfileURI:  "https://capns.org/schema/xml",
			Description: "XML document.",
			Extensions:  []string{"xml"},
		},
		{
			Urn:         "media:json;textable;form=map",
			MediaType:   "application/json",
			Title:       "JSON",
			ProfileURI:  "https://capns.org/schema/json",
			Description: "JSON data.",
			Extensions:  []string{"json"},
		},
		{
			Urn:         "media:yaml;textable;form=map",
			MediaType:   "text/yaml",
			Title:       "YAML",
			ProfileURI:  "https://capns.org/schema/yaml",
			Description: "YAML data.",
			Extensions:  []string{"yaml", "yml"},
		},
		// Media types
		{
			Urn:         "media:image;png;bytes",
			MediaType:   "image/png",
			Title:       "PNG Image",
			ProfileURI:  "https://capns.org/schema/image",
			Description: "PNG image data.",
			Extensions:  []string{"png"},
		},
		{
			Urn:         "media:image;jpeg;bytes",
			MediaType:   "image/jpeg",
			Title:       "JPEG Image",
			ProfileURI:  "https://capns.org/schema/image",
			Description: "JPEG image data.",
			Extensions:  []string{"jpg", "jpeg"},
		},
		{
			Urn:         "media:audio;wav;bytes",
			MediaType:   "audio/wav",
			Title:       "WAV Audio",
			ProfileURI:  "https://capns.org/schema/audio",
			Description: "WAV audio data.",
			Extensions:  []string{"wav"},
		},
		{
			Urn:         "media:video;bytes",
			MediaType:   "video/mp4",
			Title:       "Video",
			ProfileURI:  "https://capns.org/schema/video",
			Description: "Video data.",
			Extensions:  []string{"mp4"},
		},
	}
}
