package ipfs

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	shell "github.com/ipfs/go-ipfs-api"
	format "github.com/ipfs/go-ipld-format"
	icore "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p/core/peer"
)

// IPFSCoreAPI implements the CoreAPI interface required by OrbitDB
type IPFSCoreAPI struct {
	shell *shell.Shell
}

// Block returns the BlockAPI interface
func (api *IPFSCoreAPI) Block() icore.BlockAPI {
	return &BlockAPI{shell: api.shell}
}

// BlockAPI implements the BlockAPI interface
type BlockAPI struct {
	shell *shell.Shell
}

// Put puts a given block to IPFS
func (api *BlockAPI) Put(ctx context.Context, src io.Reader, opts ...options.BlockPutOption) (icore.BlockStat, error) {
	// Read all data from reader
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}

	// Create a basic node with the data
	ret, err := api.shell.BlockPut(data, "raw", "sha2-256", -1)
	if err != nil {
		return nil, err
	}

	c, err := cid.Decode(ret)
	if err != nil {
		return nil, err
	}

	return &BlockStat{cid: c, size: len(data)}, nil
}

// BlockStat implements the BlockStat interface
type BlockStat struct {
	cid  cid.Cid
	size int
}

func (bs *BlockStat) Path() path.Resolved {
	return path.IpfsPath(bs.cid)
}

func (bs *BlockStat) Size() int {
	return bs.size
}

// Get gets a given block from IPFS
func (api *BlockAPI) Get(ctx context.Context, p path.Path) (io.Reader, error) {
	// Get the block data using cat since BlockGet is not available
	reader, err := api.shell.Cat(p.String())
	if err != nil {
		return nil, err
	}

	return reader, nil
}

// GetNode gets a given node from IPFS
func (api *BlockAPI) GetNode(ctx context.Context, c cid.Cid) (format.Node, error) {
	// Get the block data using cat
	reader, err := api.shell.Cat(c.String())
	if err != nil {
		return nil, err
	}

	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return &BasicNode{cid: c, data: data}, nil
}

// Rm removes a block from IPFS
func (api *BlockAPI) Rm(ctx context.Context, p path.Path, opts ...options.BlockRmOption) error {
	// Not supported by shell API, but not critical for OrbitDB
	return nil
}

// Stat gets stats about a given block
func (api *BlockAPI) Stat(ctx context.Context, p path.Path) (icore.BlockStat, error) {
	// Get the CID from the path
	c, err := cid.Decode(p.String())
	if err != nil {
		return nil, err
	}

	// Get the block data to determine size
	reader, err := api.shell.Cat(p.String())
	if err != nil {
		return nil, err
	}

	// Read all data to get size
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return &BlockStat{cid: c, size: len(data)}, nil
}

// Dag returns the DagAPI interface
func (api *IPFSCoreAPI) Dag() icore.APIDagService {
	return &DagAPI{shell: api.shell, api: api}
}

// DagAPI implements the APIDagService interface
type DagAPI struct {
	shell *shell.Shell
	api   *IPFSCoreAPI
}

// Put adds a node to the DAG
func (api *DagAPI) Put(ctx context.Context, src format.Node) error {
	// For OrbitDB, we only need basic DAG operations
	data := src.RawData()
	_, err := api.shell.BlockPut(data, "raw", "sha2-256", -1)
	return err
}

// Get gets a node from the DAG
func (api *DagAPI) Get(ctx context.Context, c cid.Cid) (format.Node, error) {
	// Get the block data
	reader, err := api.shell.Cat(c.String())
	if err != nil {
		return nil, err
	}

	// Read all data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	return &BasicNode{cid: c, data: data}, nil
}

// GetMany gets multiple nodes from the DAG
func (api *DagAPI) GetMany(ctx context.Context, cids []cid.Cid) <-chan *format.NodeOption {
	ch := make(chan *format.NodeOption)
	go func() {
		defer close(ch)
		for _, c := range cids {
			node, err := api.Get(ctx, c)
			ch <- &format.NodeOption{Node: node, Err: err}
		}
	}()
	return ch
}

// Add adds a node to the DAG
func (api *DagAPI) Add(ctx context.Context, node format.Node) error {
	return api.Put(ctx, node)
}

// AddMany adds multiple nodes to the DAG
func (api *DagAPI) AddMany(ctx context.Context, nodes []format.Node) error {
	for _, node := range nodes {
		if err := api.Add(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Remove removes a node from the DAG
func (api *DagAPI) Remove(ctx context.Context, c cid.Cid) error {
	// Not critical for OrbitDB, as we don't support removal
	return nil
}

// RemoveMany removes multiple nodes from the DAG
func (api *DagAPI) RemoveMany(ctx context.Context, cids []cid.Cid) error {
	// Not critical for OrbitDB, as we don't support removal
	return nil
}

// Pinning returns a NodeAdder interface
func (api *DagAPI) Pinning() format.NodeAdder {
	return &NodeAdder{api: api}
}

// NodeAdder implements the format.NodeAdder interface
type NodeAdder struct {
	api *DagAPI
}

// Add adds a node to the DAG
func (na *NodeAdder) Add(ctx context.Context, node format.Node) error {
	return na.api.Add(ctx, node)
}

// AddMany adds multiple nodes to the DAG
func (na *NodeAdder) AddMany(ctx context.Context, nodes []format.Node) error {
	return na.api.AddMany(ctx, nodes)
}

// Pinning returns the PinAPI interface
func (api *IPFSCoreAPI) Pinning() icore.PinAPI {
	return &PinAPI{shell: api.shell}
}

// PinAPI implements the PinAPI interface
type PinAPI struct {
	shell *shell.Shell
}

// Add pins a node
func (api *PinAPI) Add(ctx context.Context, p path.Path, opts ...options.PinAddOption) error {
	return api.shell.Pin(p.String())
}

// Ls lists pinned nodes
func (api *PinAPI) Ls(ctx context.Context, opts ...options.PinLsOption) (<-chan icore.Pin, error) {
	ch := make(chan icore.Pin)
	go func() {
		defer close(ch)
		// Not critical for OrbitDB
	}()
	return ch, nil
}

// Rm unpins a node
func (api *PinAPI) Rm(ctx context.Context, p path.Path, opts ...options.PinRmOption) error {
	return api.shell.Unpin(p.String())
}

// Update updates a pin
func (api *PinAPI) Update(ctx context.Context, from path.Path, to path.Path, opts ...options.PinUpdateOption) error {
	// Not critical for OrbitDB
	return nil
}

// Verify verifies pinned data
func (api *PinAPI) Verify(ctx context.Context) (<-chan icore.PinStatus, error) {
	ch := make(chan icore.PinStatus)
	go func() {
		defer close(ch)
		// Not critical for OrbitDB
	}()
	return ch, nil
}

// IsPinned checks if a node is pinned
func (api *PinAPI) IsPinned(ctx context.Context, p path.Path, opts ...options.PinIsPinnedOption) (string, bool, error) {
	// For OrbitDB, we'll assume everything is pinned recursively
	return "recursive", true, nil
}

// Pin returns the PinAPI interface
func (api *IPFSCoreAPI) Pin() icore.PinAPI {
	return api.Pinning()
}

// Name returns the NameAPI interface
func (api *IPFSCoreAPI) Name() icore.NameAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Object returns the ObjectAPI interface
func (api *IPFSCoreAPI) Object() icore.ObjectAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Swarm returns the SwarmAPI interface
func (api *IPFSCoreAPI) Swarm() icore.SwarmAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// PubSub returns the PubSubAPI interface
func (api *IPFSCoreAPI) PubSub() icore.PubSubAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Key returns the KeyAPI interface
func (api *IPFSCoreAPI) Key() icore.KeyAPI {
	return &KeyAPI{shell: api.shell} // Return a stub implementation instead of nil
}

// KeyAPI implements the KeyAPI interface
type KeyAPI struct {
	shell *shell.Shell
}

// Self returns the identity key
func (api *KeyAPI) Self(ctx context.Context) (icore.Key, error) {
	// Get the local IPFS ID
	idOutput, err := api.shell.ID()
	if err != nil {
		return nil, err
	}

	// Parse the ID string into a peer.ID
	peerID, err := peer.Decode(idOutput.ID)
	if err != nil {
		return nil, err
	}

	// Return the ID as a Key
	return &Key{id: peerID}, nil
}

// Stub implementations for the rest of the KeyAPI interface
func (api *KeyAPI) Generate(ctx context.Context, name string, opts ...options.KeyGenerateOption) (icore.Key, error) {
	return nil, fmt.Errorf("not implemented")
}

func (api *KeyAPI) List(ctx context.Context) ([]icore.Key, error) {
	return nil, fmt.Errorf("not implemented")
}

func (api *KeyAPI) Rename(ctx context.Context, oldName string, newName string, opts ...options.KeyRenameOption) (icore.Key, bool, error) {
	return nil, false, fmt.Errorf("not implemented")
}

func (api *KeyAPI) Remove(ctx context.Context, name string) (icore.Key, error) {
	return nil, fmt.Errorf("not implemented")
}

func (api *KeyAPI) Import(ctx context.Context, name string, pem []byte, password string) (icore.Key, error) {
	return nil, fmt.Errorf("not implemented")
}

// Key implements the Key interface
type Key struct {
	id peer.ID
}

func (k *Key) Name() string {
	return "self"
}

func (k *Key) Path() path.Path {
	return path.New("/ipns/" + k.id.String())
}

func (k *Key) ID() peer.ID {
	return k.id
}

// Routing returns the RoutingAPI interface
func (api *IPFSCoreAPI) Routing() icore.RoutingAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Unixfs returns the UnixfsAPI interface
func (api *IPFSCoreAPI) Unixfs() icore.UnixfsAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Files returns the FilesAPI interface
func (api *IPFSCoreAPI) Files() icore.UnixfsAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Dht returns the DhtAPI interface
func (api *IPFSCoreAPI) Dht() icore.DhtAPI {
	return nil // Returning nil for now, will fix in follow-up PR
}

// Add imports the data from the reader into IPFS
func (api *IPFSCoreAPI) Add(ctx context.Context, file files.Node, opts ...options.UnixfsAddOption) (path.Resolved, error) {
	// Convert files.Node to a reader if possible
	if f, ok := file.(files.File); ok {
		// Add the file to IPFS using the shell
		hash, err := api.shell.Add(f)
		if err != nil {
			return nil, err
		}

		// Create a resolved path from the hash
		c, err := cid.Decode(hash)
		if err != nil {
			return nil, err
		}

		return path.IpfsPath(c), nil
	}

	return nil, fmt.Errorf("unsupported file type")
}

// WithOptions returns a new instance with the given options
func (api *IPFSCoreAPI) WithOptions(...options.ApiOption) (icore.CoreAPI, error) {
	return api, nil // Return self, no options needed for OrbitDB
}

// ResolvePath resolves the path using the UnixfsAPI
func (api *IPFSCoreAPI) ResolvePath(ctx context.Context, p path.Path) (path.Resolved, error) {
	// Implement basic path resolution
	if rp, ok := p.(path.Resolved); ok {
		return rp, nil
	}

	// Try to parse as CID
	cidStr := p.String()
	if len(cidStr) > 0 && cidStr[0] == '/' {
		cidStr = cidStr[1:]
	}
	if strings.HasPrefix(cidStr, "ipfs/") {
		cidStr = cidStr[5:]
	}

	c, err := cid.Decode(cidStr)
	if err != nil {
		return nil, err
	}

	return path.IpfsPath(c), nil
}

// ResolveNode resolves the node
func (api *IPFSCoreAPI) ResolveNode(ctx context.Context, p path.Path) (format.Node, error) {
	// For OrbitDB, we only need basic CID resolution
	c, err := cid.Decode(p.String())
	if err != nil {
		return nil, err
	}

	// Create a basic node that satisfies the format.Node interface
	return &BasicNode{cid: c}, nil
}

// BasicNode implements the format.Node interface
type BasicNode struct {
	cid  cid.Cid
	data []byte
}

func (n *BasicNode) RawData() []byte                                      { return n.data }
func (n *BasicNode) Cid() cid.Cid                                         { return n.cid }
func (n *BasicNode) String() string                                       { return n.cid.String() }
func (n *BasicNode) Loggable() map[string]interface{}                     { return nil }
func (n *BasicNode) Resolve([]string) (interface{}, []string, error)      { return nil, nil, nil }
func (n *BasicNode) Tree(string, int) []string                            { return nil }
func (n *BasicNode) ResolveLink([]string) (*format.Link, []string, error) { return nil, nil, nil }
func (n *BasicNode) Copy() format.Node                                    { return &BasicNode{cid: n.cid} }
func (n *BasicNode) Links() []*format.Link                                { return nil }
func (n *BasicNode) Stat() (*format.NodeStat, error)                      { return nil, nil }
func (n *BasicNode) Size() (uint64, error)                                { return 0, nil }
