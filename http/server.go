package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/julienschmidt/httprouter"
	peer "github.com/ontio/ontology_simulator/p2pserver/peer"
)

// Server is an HTTP server providing an API to manage a simulation network
type Server struct {
	router  *httprouter.Router
	network *peer.Network
}

// NewServer returns a new simulation API server
func NewServer(network *peer.Network) *Server {
	s := &Server{
		router:  httprouter.New(),
		network: network,
	}

	s.OPTIONS("/", s.Options)
	s.GET("/", s.GetNetwork)
	s.POST("/start", s.StartNetwork)
	s.POST("/stop", s.StopNetwork)
	s.POST("/nodes", s.CreateNode)
	s.GET("/nodes", s.GetNodes)
	s.GET("/nodes/:nodeid", s.GetNode)
	s.POST("/nodes/:nodeid/start", s.StartNode)
	s.POST("/nodes/:nodeid/stop", s.StopNode)
	s.POST("/nodes/:nodeid/conn/:peerid", s.ConnectNode)
	s.DELETE("/nodes/:nodeid/conn/:peerid", s.DisconnectNode)

	return s
}

// GetNetwork returns details of the network
func (s *Server) GetNetwork(w http.ResponseWriter, req *http.Request) {
	s.JSON(w, http.StatusOK, s.network)
}

// StartNetwork starts all nodes in the network
func (s *Server) StartNetwork(w http.ResponseWriter, req *http.Request) {
	if err := s.network.StartAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// StopNetwork stops all nodes in the network
func (s *Server) StopNetwork(w http.ResponseWriter, req *http.Request) {
	if err := s.network.StopAll(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateNode creates a node in the network using the given configuration
func (s *Server) CreateNode(w http.ResponseWriter, req *http.Request) {
	config := peer.RandomNodeConfig()
	err := json.NewDecoder(req.Body).Decode(config)
	if err != nil && err != io.EOF {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	node, err := s.network.NewNodeWithConfig(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusCreated, &node.ID)
}

// GetNodes returns all nodes which exist in the network
func (s *Server) GetNodes(w http.ResponseWriter, req *http.Request) {
	nodes := s.network.GetNodes()

	infos := make([]string, len(nodes))
	for i, node := range nodes {
		infos[i] = node.ID
	}

	s.JSON(w, http.StatusOK, infos)
}

// GetNode returns details of a node
func (s *Server) GetNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*peer.Node)

	s.JSON(w, http.StatusOK, node.ID)
}

// StartNode starts a node
func (s *Server) StartNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*peer.Node)

	if err := s.network.Start(node.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.ID)
}

// StopNode stops a node
func (s *Server) StopNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*peer.Node)

	if err := s.network.Stop(node.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.ID)
}

// ConnectNode connects a node to a peer node
func (s *Server) ConnectNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*peer.Node)
	peer := req.Context().Value("peer").(*peer.Node)

	if err := s.network.Connect(node.ID, peer.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.ID)
}

// DisconnectNode disconnects a node from a peer node
func (s *Server) DisconnectNode(w http.ResponseWriter, req *http.Request) {
	node := req.Context().Value("node").(*peer.Node)
	peer := req.Context().Value("peer").(*peer.Node)

	if err := s.network.Disconnect(node.ID, peer.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.JSON(w, http.StatusOK, node.ID)
}

// Options responds to the OPTIONS HTTP method by returning a 200 OK response
// with the "Access-Control-Allow-Headers" header set to "Content-Type"
func (s *Server) Options(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusOK)
}

// ServeHTTP implements the http.Handler interface by delegating to the
// underlying httprouter.Router
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	s.router.ServeHTTP(w, req)
}

// GET registers a handler for GET requests to a particular path
func (s *Server) GET(path string, handle http.HandlerFunc) {
	s.router.GET(path, s.wrapHandler(handle))
}

// POST registers a handler for POST requests to a particular path
func (s *Server) POST(path string, handle http.HandlerFunc) {
	s.router.POST(path, s.wrapHandler(handle))
}

// DELETE registers a handler for DELETE requests to a particular path
func (s *Server) DELETE(path string, handle http.HandlerFunc) {
	s.router.DELETE(path, s.wrapHandler(handle))
}

// OPTIONS registers a handler for OPTIONS requests to a particular path
func (s *Server) OPTIONS(path string, handle http.HandlerFunc) {
	s.router.OPTIONS("/*path", s.wrapHandler(handle))
}

// JSON sends "data" as a JSON HTTP response
func (s *Server) JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// wrapHandler returns a httprouter.Handle which wraps a http.HandlerFunc by
// populating request.Context with any objects from the URL params
func (s *Server) wrapHandler(handler http.HandlerFunc) httprouter.Handle {
	return func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		ctx := context.Background()

		if id := params.ByName("nodeid"); id != "" {
			node := s.network.GetNode(id)
			if node == nil {
				http.NotFound(w, req)
				return
			}
			ctx = context.WithValue(ctx, "node", node)
		}

		if id := params.ByName("peerid"); id != "" {
			peer := s.network.GetNode(id)
			if peer == nil {
				http.NotFound(w, req)
				return
			}
			ctx = context.WithValue(ctx, "peer", peer)
		}

		handler(w, req.WithContext(ctx))
	}
}
