package image

import "fmt"

var (
	// ManifestListMimeType is the mime type used to get the manifest list (Docker)
	ManifestListMimeType = "application/vnd.docker.distribution.manifest.list.v2+json"

	// ManifestMimeType is the mime type used to get the manifest (Docker)
	ManifestMimeType = "application/vnd.docker.distribution.manifest.v2+json"

	// OCIIndexMimeType is the mime type for OCI image indexes (multi-arch)
	OCIIndexMimeType = "application/vnd.oci.image.index.v1+json"

	// OCIManifestMimeType is the mime type for OCI image manifests
	OCIManifestMimeType = "application/vnd.oci.image.manifest.v1+json"

	// AcceptManifestList is the Accept header value for manifest list requests
	AcceptManifestList = ManifestListMimeType + ", " + OCIIndexMimeType

	// AcceptManifest is the Accept header value for manifest requests
	AcceptManifest = ManifestMimeType + ", " + OCIManifestMimeType
)

// ManifestList represents the Docker Manifest List:
// * https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-2.md
// * application/vnd.docker.distribution.manifest.list.v2+json
type ManifestList struct {
	Manifests []PlatformManifest `json:"manifests"`
}

// PlatformManifest represents an entry in a Manifest List
type PlatformManifest struct {
	*ManifestLayer
	Platform Platform `json:"platform"`
}

// Platform represents the platform description in a PlatformManifest
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}

func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Architecture)
}

// Manifest represents a Docker Image Manifest
// * https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-2.md
// * application/vnd.docker.distribution.manifest.v2+json
type Manifest struct {
	Digest        string
	SchemaVersion int             `json:"schemaVersion"`
	MediaType     string          `json:"mediaType"`
	Layers        []ManifestLayer `json:"layers"`
}

// ManifestLayer represents a Docker Image Layer
type ManifestLayer struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}
