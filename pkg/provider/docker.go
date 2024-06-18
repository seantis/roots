package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"

	"github.com/seantis/roots/pkg/image"
)

// DockerProvider authenticates clients against the Docker Hub
type DockerProvider struct {
	clients map[string]*http.Client
	mu      sync.Mutex
}

type dockerTokenResponse struct {
	Token string `json:"token"`
}

var dockerhosts = regexp.MustCompile(`([a-z0-9-]+\.)?docker\.io`)

func init() {
	image.RegisterProvider("docker", &DockerProvider{
		clients: make(map[string]*http.Client),
	})
}

// Supports returns true if the URLs host is one of the google cloud registry hosts
func (p *DockerProvider) Supports(url image.URL) bool {
	return dockerhosts.MatchString(url.Host)
}

// GetClient returns a client authenticated with the Docker Hub. Currently
// there's no support for private repositories and 'auth' is ignored. Note also
// that the token given by Docker Hub expires after 5 minutes - renewal logic
// has not been implemented yet.
func (p *DockerProvider) GetClient(url image.URL, auth string) (*http.Client, error) {

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

// newClient returns a new client authenitcated with the Docker Hub
func (p *DockerProvider) newClient(repository string, name string, auth string) (*http.Client, error) {
	// even public api connections need an authorization token
	t := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s/%s:pull"
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
