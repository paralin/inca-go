package node

import (
	"context"

	// inet "github.com/libp2p/go-libp2p-net"
	icore "github.com/ipfs/go-ipfs/core"
	"github.com/jbenet/goprocess"
	ma "github.com/multiformats/go-multiaddr"
)

// Node is an instance of an Inca p2p node.
type Node struct {
	*icore.IpfsNode
	process goprocess.Process
}

// NewNode builds the p2p node.
func NewNode(
	ctx context.Context,
	config *NodeConfig,
) (*Node, error) {
	if config == nil {
		config = &NodeConfig{}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	config.FillDefaults()
	config.FillMandatory()

	ipfsNode, err := icore.NewNode(ctx, config.BuildCfg)
	if err != nil {
		return nil, err
	}

	n := &Node{IpfsNode: ipfsNode}
	n.process = goprocess.Go(n.nodeProcess)
	return n, nil
}
