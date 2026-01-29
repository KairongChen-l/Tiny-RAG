// Package ingestion provides document parsing and processing.
package ingestion

// DocumentFormat represents supported document formats.
type DocumentFormat string

const (
	FormatMarkdown   DocumentFormat = "markdown"
	FormatPDF        DocumentFormat = "pdf"
	FormatText       DocumentFormat = "text"
	FormatWord       DocumentFormat = "word"        // DOCX, DOC
	FormatExcel      DocumentFormat = "excel"       // XLSX, XLS
	FormatPowerPoint DocumentFormat = "powerpoint" // PPTX, PPT
	FormatHTML       DocumentFormat = "html"        // HTML, HTM
	FormatXML        DocumentFormat = "xml"        // XML
	FormatRTF        DocumentFormat = "rtf"        // RTF
	FormatODT        DocumentFormat = "odt"        // ODT
)

// Document represents a parsed document with hierarchical structure.
type Document struct {
	ID       string            // Unique identifier
	Title    string            // Document title
	Source   string            // Original file path
	Format   DocumentFormat    // Document format
	Sections []Section         // Hierarchical sections
	Metadata map[string]string // Custom metadata
	Hash     string            // Content hash for incremental detection
}

// Section represents a document section with nested structure.
type Section struct {
	Title    string    // Section title (heading text)
	Level    int       // Heading level (1=H1, 2=H2, etc.)
	Content  string    // Section content (text under this heading)
	Children []Section // Nested subsections
}

// NewDocument creates a new Document with initialized fields.
func NewDocument(id, source string, format DocumentFormat) *Document {
	return &Document{
		ID:       id,
		Source:   source,
		Format:   format,
		Sections: make([]Section, 0),
		Metadata: make(map[string]string),
	}
}

// AddMetadata adds a metadata key-value pair to the document.
func (d *Document) AddMetadata(key, value string) {
	if d.Metadata == nil {
		d.Metadata = make(map[string]string)
	}
	d.Metadata[key] = value
}

// GetAllContent returns all text content from the document.
func (d *Document) GetAllContent() string {
	var content string
	for _, section := range d.Sections {
		content += section.getAllContent()
	}
	return content
}

// getAllContent recursively collects content from a section and its children.
func (s *Section) getAllContent() string {
	content := s.Content
	for _, child := range s.Children {
		content += "\n" + child.getAllContent()
	}
	return content
}

// FlattenSections returns all sections as a flat slice with their paths.
type FlatSection struct {
	Section
	Path string // e.g., "Chapter1/Section2/Subsection1"
}

// Flatten returns all sections in a flat structure with their paths.
func (d *Document) Flatten() []FlatSection {
	var result []FlatSection
	for _, section := range d.Sections {
		result = append(result, flattenSection(section, "")...)
	}
	return result
}

func flattenSection(s Section, parentPath string) []FlatSection {
	path := s.Title
	if parentPath != "" {
		path = parentPath + "/" + s.Title
	}

	result := []FlatSection{{Section: s, Path: path}}
	for _, child := range s.Children {
		result = append(result, flattenSection(child, path)...)
	}
	return result
}

