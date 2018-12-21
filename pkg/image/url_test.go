package image

import (
	"fmt"
	"testing"

	"github.com/seantis/roots/pkg/image"
	"github.com/stretchr/testify/assert"
)

var cases = []struct {
	url      string
	expected image.URL
	format   string
}{
	{
		"ubuntu", image.URL{
			Name:       "ubuntu",
			Tag:        "latest",
			Repository: "library",
			Host:       "registry-1.docker.io",
		},
		"registry-1.docker.io/library/ubuntu:latest",
	},
	{
		"ubuntu:18.04", image.URL{
			Name:       "ubuntu",
			Tag:        "18.04",
			Repository: "library",
			Host:       "registry-1.docker.io",
		},
		"registry-1.docker.io/library/ubuntu:18.04",
	},
	{
		"gcr.io/google-containers/ubuntu", image.URL{
			Name:       "ubuntu",
			Tag:        "latest",
			Repository: "google-containers",
			Host:       "gcr.io",
		},
		"gcr.io/google-containers/ubuntu:latest",
	},
	{
		"foo/bar", image.URL{
			Name:       "bar",
			Tag:        "latest",
			Repository: "foo",
			Host:       "registry-1.docker.io",
		},
		"registry-1.docker.io/foo/bar:latest",
	},
	{
		"foo/bar@sha256:0xdeadbeef", image.URL{
			Name:       "bar",
			Tag:        "latest",
			Repository: "foo",
			Host:       "registry-1.docker.io",
			Digest:     "sha256:0xdeadbeef",
		},
		"registry-1.docker.io/foo/bar:latest@sha256:0xdeadbeef",
	},
	{
		"", image.URL{}, "<empty>",
	},
	{
		"@", image.URL{}, "<empty>",
	},
	{
		"/////@@", image.URL{}, "<empty>",
	},
	{
		"    ", image.URL{}, "<empty>",
	},
}

// TestParse tests the image URL parsing
func TestParse(t *testing.T) {
	for _, c := range cases {
		t.Run(c.url, func(t *testing.T) {
			result, _ := image.Parse(c.url)

			assert.Equal(t, c.expected, *result, "unexpected url")

			format := fmt.Sprintf("%s", result)
			assert.Equal(t, format, c.format, "unexpected format")
		})
	}
}
