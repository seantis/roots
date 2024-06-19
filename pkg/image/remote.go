package image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// Remote represents an image on a remote repository
type Remote struct {
	client   *http.Client
	url      URL
	platform *Platform
	ctx      context.Context
}

func (r *Remote) String() string {
	if r.platform != nil {
		return fmt.Sprintf("%s %s", r.url, r.platform)
	}

	return r.url.String()
}

// NewRemote returns a new remote instance. An error is returned if the
// remote instance cannot be accessed due to lack of permissions.
func NewRemote(ctx context.Context, url URL, auth string) (*Remote, error) {
	provider, err := LookupProvider(url)
	if err != nil {
		return nil, err
	}

	client, err := provider.GetClient(url, auth)
	if err != nil {
		return nil, err
	}

	err = requireSupportedMimeTypes(client, url)
	if err != nil {
		return nil, err
	}

	return &Remote{
		url:    url,
		client: client,
		ctx:    ctx,
	}, nil
}

// Platforms returns all the platforms the image supports. Nil is is
// returned if the image does not have multi-platform support (i.e. there is
// no manifest list).
//
// If the image has platforms, you should bind the required platform to the
// Remote using WithPlatform, before using other methods, as you will otherwise
// get whatever the registry deems to be the default platform of the manifest,
// which might not be what you want.
func (r *Remote) Platforms() ([]*Platform, error) {

	// try to get the manifest list (not all images have this)
	l, err := r.ManifestList()
	if err != nil || l == nil {
		return nil, err
	}

	// each manifest has exactly one platform
	platforms := make([]*Platform, len(l.Manifests))
	for i, m := range l.Manifests {
		platforms[i] = &m.Platform
	}

	return platforms, nil
}

// WithPlatform binds the given platform to the remote and uses it to
// scope the Digest and Manifest methods
func (r *Remote) WithPlatform(p *Platform) {
	r.platform = p
}

// ManifestList queries the remote for the manifest list and parses the result.
// If the manifest list does not exist, the method returns nil, nil instead of
// an error, as manifest lists are not available for most images today.
func (r *Remote) ManifestList() (*ManifestList, error) {

	// not having a manifest list is no error
	res, err := r.request("GET", ManifestListMimeType, "manifests", r.url.Reference())
	if err != nil {
		return nil, nil
	}

	// not being able to parse an existing list is however
	lst := &ManifestList{}
	if err := r.unmarshal(res, lst); err != nil {
		return nil, fmt.Errorf("error parsing manifest list: %v", err)
	}

	return lst, nil
}

// Manifest gets the manifest of the image. The current platform is
// respected if one was set through WithPlatform.
func (r *Remote) Manifest() (*Manifest, error) {

	// the digest is bound to the platform
	digest, err := r.Digest()
	if err != nil {
		return nil, err
	}

	// it should almost certainly be fetchable at this point
	res, err := r.request("GET", ManifestMimeType, "manifests", digest)
	if err != nil {
		return nil, fmt.Errorf("error requesting manifest@%s: %v", digest, err)
	}

	// if the server responds with a manifest list, our digest is not correct
	if res.Header.Get("Content-Type") != ManifestMimeType {
		return nil, fmt.Errorf("content type for %s cannot be %s", digest, res.Header.Get("Content-Type"))
	}

	// we must also be able to parse it
	m := &Manifest{Digest: digest}
	if err := r.unmarshal(res, &m); err != nil {
		return nil, fmt.Errorf("error parsing manifest: %v", err)
	}

	return m, nil
}

// Digest gets the latest digest of the image. The current platform is
// respected if one was set through WithPlatform.
func (r *Remote) Digest() (string, error) {
	// due to https://github.com/docker/distribution/issues/2395 we always
	// have to request the manifest list, even if it doesn't exist, as images
	// with manifest lists on docker hub will not return the expected digest
	lst, err := r.ManifestList()
	if err != nil {
		return "", err
	}

	// if there's a list, but no platform, take the first item
	//
	// we could be cleverer here by picking the platform or we could let
	// the user know that he should pick one
	if r.platform == nil && lst != nil && len(lst.Manifests) != 0 {
		return lst.Manifests[0].Digest, nil
	}

	// if there's no list and no platform, fall back to whatever the server
	// gives us through the docker-content-digest header
	if r.platform == nil && (lst == nil || len(lst.Manifests) == 0) {
		res, err := r.request("HEAD", ManifestMimeType, "manifests", r.url.Reference())

		if err != nil {
			return "", fmt.Errorf("failed to fetch manifest: %v", err)
		}

		return res.Header.Get("Docker-Content-Digest"), nil
	}

	// if there is a platform, we require a list
	if lst == nil {
		return "", fmt.Errorf("no multi-platform support: %s", r.url)
	}

	for _, m := range lst.Manifests {
		if m.Platform == *r.platform {
			return m.Digest, nil
		}
	}

	// there was no match
	return "", fmt.Errorf("no manifest found for %s", r)
}

// Layers returns the layers of the image. The current plaform is
func (r *Remote) Layers() ([]ManifestLayer, error) {

	m, err := r.Manifest()
	if err != nil {
		return nil, err
	}

	return m.Layers, nil
}

// DownloadLayer downloads a layer to a Writer
func (r *Remote) DownloadLayer(digest string, w io.Writer) error {

	res, err := r.request("GET", "*", "blobs", digest)
	if err != nil {
		return fmt.Errorf("failed to download %s: %v", digest, err)
	}

	// copy the downloads using the default buffer
	defer res.Body.Close()

	_, err = io.Copy(w, res.Body)
	if err != nil {
		return fmt.Errorf("error downloading %s: %v", digest, err)
	}

	return nil
}

func (r *Remote) request(method string, accept string, segments ...string) (*http.Response, error) {
	req, err := http.NewRequest(method, r.url.Endpoint(segments...), nil)
	if err != nil {
		return nil, fmt.Errorf("error requesting %s: %v", req.URL, err)
	}

	req = req.WithContext(r.ctx)

	req.Header.Add("Accept", accept)
	res, err := r.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("error requesting %s: %v", req.URL, err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("%s %s failed with %s", method, req.URL, res.Status)
	}

	return res, nil
}

func (r *Remote) unmarshal(res *http.Response, v interface{}) error {
	body, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	err = json.Unmarshal(body, &v)
	if err != nil {
		return fmt.Errorf("error unmarshaling response into %v: %v", v, err)
	}

	return nil
}
