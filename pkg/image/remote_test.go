package image

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/dankinder/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockProvider struct {
	Server *httpmock.Server
}

func (p *mockProvider) GetClient(url URL, auth string) (*http.Client, error) {
	return http.DefaultClient, nil
}

func (p *mockProvider) Supports(url URL) bool {
	return true
}

func mockServer() *httpmock.Server {
	downstream := &httpmock.MockHandler{}

	header := make(http.Header)
	header.Add("Docker-Content-Digest", "foobar")
	header.Add("Content-Type", ManifestListMimeType)

	downstream.On("Handle", "HEAD", "/v2/library/ubuntu/manifests/latest", mock.Anything).Return(httpmock.Response{
		Header: header,
	})

	downstream.On("Handle", "GET", "/v2/library/ubuntu/manifests/latest", mock.Anything).Return(httpmock.Response{
		Header: header,
		Body: []byte(`
			{
				"schemaVersion": 2,
				"mediaType": "application/vnd.docker.distribution.manifest.list.v2+json",
				"manifests": [
						{
							"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
							"size": 123,
							"digest": "foobar",
							"platform": {
									"architecture": "amd64",
									"os": "linux"
								}
						}
					]
				}
		`),
	})

	return httpmock.NewServer(downstream)
}

// TestRemoteDigest tests the lookup of the digest on a mock provider
func TestRemoteDigest(t *testing.T) {
	defer ClearProviderRegistry()

	server := mockServer()
	defer server.Close()

	RegisterProvider("mock", &mockProvider{
		Server: server,
	})

	url := URL{
		Host:       server.URL(),
		Name:       "ubuntu",
		Repository: "library",
		Tag:        "latest",
	}

	remote, _ := NewRemote(context.Background(), url, "")

	digest, err := remote.Digest()
	assert.NoError(t, err, "error during mock lookup")
	assert.Equal(t, "foobar", digest, "could not lookup mock digest")

	remote.WithPlatform(&Platform{
		Architecture: "arm",
		OS:           "linux",
	})
	digest, err = remote.Digest()
	assert.EqualError(t, err, fmt.Sprintf("no manifest found for %s linux/arm", url), "unexpected error")
	assert.Equal(t, "", digest, "could not lookup mock digest")
}
