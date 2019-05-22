package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	peer "github.com/ontio/ontology_simulator/p2pserver/peer"
)

// DefaultClient is the default simulation API client which expects the API
// to be running at http://localhost:8888
var DefaultClient = NewClient("http://localhost:8888")

// Client is a client for the simulation HTTP API which supports creating
// and managing simulation networks
type Client struct {
	URL string

	client *http.Client
}

// NewClient returns a new simulation API client
func NewClient(url string) *Client {
	return &Client{
		URL:    url,
		client: http.DefaultClient,
	}
}

// GetNetwork returns details of the network
func (c *Client) GetNetwork() (*peer.Network, error) {
	network := &peer.Network{}
	return network, c.Get("/", network)
}

// StartNetwork starts all existing nodes in the simulation network
func (c *Client) StartNetwork() error {
	return c.Post("/start", nil, nil)
}

// StopNetwork stops all existing nodes in a simulation network
func (c *Client) StopNetwork() error {
	return c.Post("/stop", nil, nil)
}

// GetNodes returns all nodes which exist in the network
func (c *Client) GetNodes() ([]string, error) {
	var nodes []string
	return nodes, c.Get("/nodes", &nodes)
}

// CreateNode creates a node in the network using the given configuration
func (c *Client) CreateNode(config *peer.NodeConfig) (string, error) {
	node := ""
	return node, c.Post("/nodes", config, node)
}

// StartNode starts a node
func (c *Client) StartNode(nodeID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/start", nodeID), nil, nil)
}

// StopNode stops a node
func (c *Client) StopNode(nodeID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/stop", nodeID), nil, nil)
}

// ConnectNode connects a node to a peer node
func (c *Client) ConnectNode(nodeID, peerID string) error {
	return c.Post(fmt.Sprintf("/nodes/%s/conn/%s", nodeID, peerID), nil, nil)
}

// DisconnectNode disconnects a node from a peer node
func (c *Client) DisconnectNode(nodeID, peerID string) error {
	return c.Delete(fmt.Sprintf("/nodes/%s/conn/%s", nodeID, peerID))
}

// Get performs a HTTP GET request decoding the resulting JSON response
// into "out"
func (c *Client) Get(path string, out interface{}) error {
	return c.Send("GET", path, nil, out)
}

// Post performs a HTTP POST request sending "in" as the JSON body and
// decoding the resulting JSON response into "out"
func (c *Client) Post(path string, in, out interface{}) error {
	return c.Send("POST", path, in, out)
}

// Delete performs a HTTP DELETE request
func (c *Client) Delete(path string) error {
	return c.Send("DELETE", path, nil, nil)
}

// Send performs a HTTP request, sending "in" as the JSON request body and
// decoding the JSON response into "out"
func (c *Client) Send(method, path string, in, out interface{}) error {
	var body []byte
	if in != nil {
		var err error
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequest(method, c.URL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		response, _ := ioutil.ReadAll(res.Body)
		return fmt.Errorf("unexpected HTTP status: %s: %s", res.Status, response)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			return err
		}
	}
	return nil
}
