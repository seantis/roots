package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"github.com/seantis/roots/pkg/image"
)

// GHProvider does not authenticate at the moment
type GHProvider struct {
	clients map[string]*http.Client
	mu      sync.Mutex
}

func init() {
	image.RegisterProvider("gh", &GHProvider{
		clients: make(map[string]*http.Client),
	})
}

var ghhosts = regexp.MustCompile(`ghcr\.io`)

// Supports returns true if the URLs host is one of the GitHub Container
// Registry hosts
func (p *GHProvider) Supports(url image.URL) bool {
	return ghhosts.MatchString(url.Host)
}

// GetClient returns a client for the GitHub Container Registry. Currently
// there's no support for private repositories and 'auth' is ignored.
func (p *GHProvider) GetClient(url image.URL, auth string) (*http.Client, error) {

	p.mu.Lock()
	defer p.mu.Unlock()

	// The client for Docker is bound to the repository
	if p.clients[url.Repository] == nil {
		client, err := p.newClient(url.Repository, url.Name, auth)

		if err != nil {
			return nil, err
		}

		p.clients[url.Repository] = client
	}

	return p.clients[url.Repository], nil
}

// newClient spawns a new unauthenticated http client for GitHub Container
// Repository
func (p *GHProvider) newClient(repository string, name string, auth string) (*http.Client, error) {
	// even public api connections need an authorization token
	t := "https://ghcr.io/token?scope=repository:%s/%s:pull"
	u := fmt.Sprintf(t, repository, name)

	res, err := http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("error getting access-token via %s: %v", u, err)
	}

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GET %s failed with %s", u, res.Status)
	}

	// we'll get it from the json response
	tr := &dockerTokenResponse{}
	err = json.NewDecoder(res.Body).Decode(&tr)

	if err != nil {
		return nil, fmt.Errorf("error parsing response: %e", err)
	}

	if len(tr.Token) == 0 {
		return nil, fmt.Errorf("%s did not return a token", u)
	}

	// we then use it to create a client with a proper bearer token set
	return clientWithHeaders(map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", tr.Token),
	}), err
}
