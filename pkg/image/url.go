package image

import (
	"fmt"
	"regexp"
	"strings"
)

var localurl = regexp.MustCompile(`(?i)^http://(127\.[\d.]+|[0:]+1|localhost)`)

// URL contains the result of a parsed container url like the following:
// * ubuntu:latest
// * gcr.io/google-containers/alpine
// * busybox:123@foobar
// See also https://stackoverflow.com/q/37861791
type URL struct {
	Name       string
	Host       string
	Repository string
	Tag        string
	Digest     string
}

// String returns the normalized form of the URL (i.e the longer form with
// a guaranteed host, repository and tag name) - if the URL is empty, "<empty>"
// is returned
func (url URL) String() string {
	if len(url.Name) == 0 {
		return "<empty>"
	}

	if len(url.Digest) == 0 {
		return fmt.Sprintf("%s/%s/%s:%s",
			url.Host,
			url.Repository,
			url.Name,
			url.Tag)
	}

	return fmt.Sprintf("%s/%s/%s:%s@%s",
		url.Host,
		url.Repository,
		url.Name,
		url.Tag,
		url.Digest)
}

// Endpoint returns an API endpoint of the v2 registry API
func (url URL) Endpoint(segments ...string) string {
	// by default, no protocol is given and we force https
	host := fmt.Sprintf("https://%s", url.Host)

	// the host may include the http protocol if it points to a local address
	if localurl.MatchString(url.Host) {
		host = url.Host
	}

	return fmt.Sprintf("%s/v2/%s/%s/%s",
		host,
		url.Repository,
		url.Name,
		strings.Join(segments, "/"))
}

// Reference returns either the digest or, if the digest is absent, the tag
func (url URL) Reference() string {
	if len(url.Digest) > 0 {
		return url.Digest
	}

	return url.Tag
}

// Parse parses the given URL and returns an error if it doesn't look correct
func Parse(url string) (*URL, error) {
	url = strings.Trim(url, " \n\t")

	if len(url) == 0 {
		return &URL{}, fmt.Errorf("passed an empty url")
	}

	p := &URL{}

	// if there's an @, we got our digest
	if strings.Contains(url, "@") {
		url, p.Digest = bisect(url, "@")
	}

	// before the slash is the host and repository, after it the name and tag
	parts := strings.Split(url, "/")

	// if there is a slash and we got a dot or a colon we found a host name
	if strings.Contains(url, "/") && strings.ContainsAny(parts[0], ".:") {
		p.Host, parts = parts[0], parts[1:]
	}

	// if there's a colon in the last part, we got a tag
	if strings.Contains(parts[len(parts)-1], ":") {
		parts[len(parts)-1], p.Tag = bisect(parts[len(parts)-1], ":")
	}

	// the rest should be the name and possibly the repository
	switch len(parts) {
	case 1:
		p.Name = parts[0]
	case 2:
		p.Repository, p.Name = parts[0], parts[1]
	default:
		return &URL{}, fmt.Errorf("too many slashes in %s", url)
	}

	if len(p.Name) == 0 {
		return &URL{}, fmt.Errorf("could not find a name for %s", url)
	}

	// finally, we add some defaults that are set in practice
	if len(p.Host) == 0 {
		p.Host = "registry-1.docker.io"
	}

	if len(p.Tag) == 0 {
		p.Tag = "latest"
	}

	if len(p.Repository) == 0 {
		p.Repository = "library"
	}

	return p, nil
}
