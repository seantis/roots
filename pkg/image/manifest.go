package image

import "fmt"

var (
    // ManifestListMimeType is the mime type used to get the manifest list
    ManifestListMimeType = "application/vnd.docker.distribution.manifest.list.v2+json"

    // ManifestMimeType is the mime type used to get the manifest
    ManifestMimeType = "application/vnd.docker.distribution.manifest.v2+json"
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
