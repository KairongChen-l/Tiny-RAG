package payloads

// DocumentIngestPayload is the job payload for document ingestion.
//
// Exactly one of LocalPath or ObjectKey should be set.
// - LocalPath: worker reads from local filesystem (typically temp file)
// - ObjectKey: worker downloads from object storage (e.g., MinIO) then parses
type DocumentIngestPayload struct {
	LocalPath string            `json:"local_path,omitempty"`
	ObjectKey string            `json:"object_key,omitempty"`
	Filename  string            `json:"filename,omitempty"`
	Metadata  map[string]string `json:"metadata"`
}


