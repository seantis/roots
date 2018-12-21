package image

import (
	"fmt"
	"net/http"
	"strings"
)

func bisect(text string, delimiter string) (string, string) {
	split := strings.SplitN(text, delimiter, 2)
	return split[0], split[1]
}

// mustNewRequest calls http.NewRequest, but panics if there's an error (as those
// are most certainly errors that we catch during testing)
func mustNewRequest(method string, url string) *http.Request {
	res, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}

	return res
}

func requireSupportedMimeTypes(client *http.Client, url URL) error {
	ref := url.Endpoint("manifests", url.Reference())

	req := mustNewRequest("HEAD", ref)
	req.Header.Add("Accept", fmt.Sprintf("%s, */*", ManifestMimeType))

	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error requesting %s: %v", ref, err)
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("HEAD %s returned %s", ref, res.Status)
	}

	mime := res.Header.Get("Content-Type")
	if mime != ManifestMimeType && mime != ManifestListMimeType {
		return fmt.Errorf("no schema version 2 support by %s", url)
	}

	return nil
}
