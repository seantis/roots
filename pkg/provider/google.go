package provider

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"sync"

	"github.com/seantis/roots/pkg/image"
	"golang.org/x/oauth2/google"
)

// GCRProvider authenticates clients against the Google Cloud Registry
type GCRProvider struct {
	clients map[string]*http.Client
	mu      sync.Mutex
}

func init() {
	image.RegisterProvider("gcr", &GCRProvider{
		clients: make(map[string]*http.Client),
	})
}

var gcrhosts = regexp.MustCompile(`([a-z]+?\.)?gcr\.io`)
var gcrscope = "https://www.googleapis.com/auth/devstorage.read_only"

// Supports returns true if the URLs host is one of the google cloud registry hosts
func (p *GCRProvider) Supports(url image.URL) bool {
	return gcrhosts.MatchString(url.Host)
}

// GetClient returns a client authenticated with the Google Cloud Registry -
// the auth string is supposed to be the path to a service account json file
// the required scope is limit to https://www.googleapis.com/auth/devstorage.read_only
func (p *GCRProvider) GetClient(url image.URL, auth string) (*http.Client, error) {

	p.mu.Lock()
	defer p.mu.Unlock()

	// The client for GCR is only bound to the auth string
	if p.clients[auth] == nil {
		client, err := p.newClient(auth)

		if err != nil {
			return nil, err
		}

		p.clients[auth] = client
	}

	return p.clients[auth], nil
}

// newClient spawns a new http client for GCR given the path to an account json
// file, or an empty string (for anonymous access)
func (p *GCRProvider) newClient(auth string) (*http.Client, error) {
	// unauthenticated access
	if len(auth) == 0 {
		return &http.Client{}, nil
	}

	json, err := ioutil.ReadFile(auth)
	if err != nil {
		return nil, fmt.Errorf("error reading auth file %s: %v", auth, err)
	}

	conf, err := google.JWTConfigFromJSON(json, gcrscope)
	if err != nil {
		return nil, fmt.Errorf("error authenticating with %s: %v", gcrscope, err)
	}

	return conf.Client(context.Background()), nil
}
