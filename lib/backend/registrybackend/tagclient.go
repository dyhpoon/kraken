package registrybackend

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/uber/kraken/core"
	"github.com/uber/kraken/lib/backend"
	"github.com/uber/kraken/lib/backend/backenderrors"
	"github.com/uber/kraken/utils/httputil"
	yaml "gopkg.in/yaml.v2"
)

const _registrytag = "registry_tag"

func init() {
	backend.Register(_registrytag, &tagClientFactory{})
}

type tagClientFactory struct{}

func (f *tagClientFactory) Create(
	confRaw interface{}, authConfRaw interface{}) (backend.Client, error) {

	confBytes, err := yaml.Marshal(confRaw)
	if err != nil {
		return nil, errors.New("marshal hdfs config")
	}
	var config Config
	if err := yaml.Unmarshal(confBytes, &config); err != nil {
		return nil, errors.New("unmarshal hdfs config")
	}
	return NewTagClient(config)
}

const _tagquery = "http://%s/v2/%s/manifests/%s"

// TagClient stats and downloads tag from registry.
type TagClient struct {
	config Config
}

// NewTagClient creates a new TagClient.
func NewTagClient(config Config) (*TagClient, error) {
	return &TagClient{config}, nil
}

// Stat sends a HEAD request to registry for a tag and returns the manifest size.
func (c *TagClient) Stat(namespace, name string) (*core.BlobInfo, error) {
	tokens := strings.Split(name, ":")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("invald name %s: must be repo:tag", name)
	}
	repo, tag := tokens[0], tokens[1]

	opt, err := c.config.Security.GetHTTPOption(c.config.Address, repo)
	if err != nil {
		return nil, fmt.Errorf("get security opt: %s", err)
	}

	URL := fmt.Sprintf(_tagquery, c.config.Address, repo, tag)
	resp, err := httputil.Head(
		URL,
		opt,
		httputil.SendAcceptedCodes(http.StatusOK, http.StatusNotFound))
	if err != nil {
		return nil, fmt.Errorf("check blob exists: %s", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, backenderrors.ErrBlobNotFound
	}
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse blob size: %s", err)
	}
	return core.NewBlobInfo(size), nil
}

// Download gets the digest for a tag from registry.
func (c *TagClient) Download(namespace, name string, dst io.Writer) error {
	tokens := strings.Split(name, ":")
	if len(tokens) != 2 {
		return fmt.Errorf("invald name %s: must be repo:tag", name)
	}
	repo, tag := tokens[0], tokens[1]

	opt, err := c.config.Security.GetHTTPOption(c.config.Address, repo)
	if err != nil {
		return fmt.Errorf("get security opt: %s", err)
	}

	URL := fmt.Sprintf(_tagquery, c.config.Address, repo, tag)
	resp, err := httputil.Head(
		URL,
		opt,
		httputil.SendAcceptedCodes(http.StatusOK, http.StatusNotFound))
	if err != nil {
		return fmt.Errorf("check blob exists: %s", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return backenderrors.ErrBlobNotFound
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return fmt.Errorf("empty Docker-Content-Digest header")
	}
	if _, err := io.Copy(dst, strings.NewReader(digest)); err != nil {
		return fmt.Errorf("copy: %s", err)
	}
	return nil
}

// Upload is not supported as users can push directly to registry.
func (c *TagClient) Upload(namespace, name string, src io.Reader) error {
	return errors.New("not supported")
}

// List is not supported as users can list directly from registry.
func (c *TagClient) List(prefix string) ([]string, error) {
	return nil, errors.New("not supported")
}
