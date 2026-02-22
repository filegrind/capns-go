// Package standard provides standard media URN constants and cap URN builders
package standard

// =============================================================================
// STANDARD MEDIA URN CONSTANTS
// =============================================================================

// MediaVoid represents the void media type
const MediaVoid = "media:void"

// MediaString represents the string media type
const MediaString = "media:string"

// MediaBinary represents the binary media type
const MediaBinary = "media:binary"

// MediaObject represents the object (map) media type
const MediaObject = "media:object"

// MediaInteger represents the integer media type
const MediaInteger = "media:integer"

// MediaNumber represents the number (float) media type
const MediaNumber = "media:number"

// MediaBoolean represents the boolean media type
const MediaBoolean = "media:boolean"

// Domain-specific media types

// MediaModelSpec represents model specification media type
const MediaModelSpec = "media:model-spec"

// MediaAvailabilityOutput represents model availability output media type
const MediaAvailabilityOutput = "media:availability-output"

// MediaPathOutput represents path output media type
const MediaPathOutput = "media:path-output"

// MediaLlmInferenceOutput represents LLM inference output media type
const MediaLlmInferenceOutput = "media:llm-inference-output"

// =============================================================================
// EXPANDED MEDIA URN CONSTANTS (with semantic tags, matching Rust)
// =============================================================================

// Scalar types with semantic tags

// MediaStringExpanded is the expanded form of MediaString with semantic tags
const MediaStringExpanded = "media:textable;form=scalar"

// MediaIntegerExpanded is the expanded form of MediaInteger with semantic tags
const MediaIntegerExpanded = "media:integer;textable;numeric;form=scalar"

// MediaNumberExpanded is the expanded form of MediaNumber with semantic tags
const MediaNumberExpanded = "media:textable;numeric;form=scalar"

// MediaBooleanExpanded is the expanded form of MediaBoolean with semantic tags
const MediaBooleanExpanded = "media:bool;textable;form=scalar"

// MediaObjectExpanded is the expanded form of MediaObject with semantic tags
const MediaObjectExpanded = "media:form=map;textable"

// MediaBinaryExpanded is the expanded form of MediaBinary with semantic tags
const MediaBinaryExpanded = "media:"

// Array types

// MediaStringArray represents a string array media type
const MediaStringArray = "media:textable;form=list"

// MediaIntegerArray represents an integer array media type
const MediaIntegerArray = "media:integer;textable;numeric;form=list"

// MediaNumberArray represents a number array media type
const MediaNumberArray = "media:textable;numeric;form=list"

// MediaBooleanArray represents a boolean array media type
const MediaBooleanArray = "media:bool;textable;form=list"

// MediaObjectArray represents an object array media type
const MediaObjectArray = "media:form=list;textable"

// Image/Audio/Video types

// MediaPng represents a PNG image media type
const MediaPng = "media:image;png"

// MediaAudio represents an audio media type
const MediaAudio = "media:wav;audio"

// MediaVideo represents a video media type
const MediaVideo = "media:video"

// MediaAudioSpeech represents audio speech media type
const MediaAudioSpeech = "media:audio;wav;speech"

// MediaImageThumbnail represents an image thumbnail media type
const MediaImageThumbnail = "media:image;png;thumbnail"

// Collection types

// MediaCollection represents a collection media type (map form)
const MediaCollection = "media:collection;textable;form=map"

// MediaCollectionList represents a collection media type (list form)
const MediaCollectionList = "media:collection;textable;form=list"

// Document types

// MediaPdf represents a PDF document media type
const MediaPdf = "media:pdf"

// MediaEpub represents an EPUB document media type
const MediaEpub = "media:epub"

// Text format types

// MediaMd represents a Markdown text media type
const MediaMd = "media:md;textable"

// MediaTxt represents a plain text media type
const MediaTxt = "media:txt;textable"

// MediaRst represents a reStructuredText media type
const MediaRst = "media:rst;textable"

// MediaLog represents a log text media type
const MediaLog = "media:log;textable"

// MediaHtml represents an HTML text media type
const MediaHtml = "media:html;textable"

// MediaXml represents an XML text media type
const MediaXml = "media:xml;textable"

// Structured text types

// MediaJson represents a JSON media type
const MediaJson = "media:json;textable;form=map"

// MediaJsonSchema represents a JSON Schema media type
const MediaJsonSchema = "media:json;json-schema;textable;form=map"

// MediaYaml represents a YAML media type
const MediaYaml = "media:yaml;textable;form=map"

// File path types

// MediaFilePath represents a file path media type (scalar)
const MediaFilePath = "media:file-path;textable;form=scalar"

// MediaFilePathArray represents a file path array media type (list)
const MediaFilePathArray = "media:file-path;textable;form=list"

// Decision types

// MediaDecision represents a decision media type
const MediaDecision = "media:decision;bool;textable;form=scalar"

// MediaDecisionArray represents a decision array media type
const MediaDecisionArray = "media:decision;bool;textable;form=list"
