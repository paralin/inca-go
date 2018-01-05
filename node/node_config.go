package node

import (
	"context"
	"crypto/rand"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	crypto "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	// inet "github.com/libp2p/go-libp2p-net"
	icore "github.com/ipfs/go-ipfs/core"
	repo "github.com/ipfs/go-ipfs/repo"
	circuit "github.com/libp2p/go-libp2p-circuit"
	peer "github.com/libp2p/go-libp2p-peer"
	ps "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	"github.com/libp2p/go-libp2p/p2p/host/basic"
	"github.com/libp2p/go-libp2p/p2p/host/routed"
	ma "github.com/multiformats/go-multiaddr"
)

// NodeConfig contains the configuration for making a new node.
type NodeConfig struct {
	// BuildCfg contains options for the IPFS node.
	*icore.BuildCfg

	// LogEntry is the root log entry for the node.
	LogEntry *logrus.Entry
	// LightClient indicates this is a light client.
	LightClient bool
}

// FillDefaults fills any unset fields.
func (n *NodeConfig) FillDefaults() {
	if n.BuildCfg == nil {
		n.BuildCfg = &icore.BuildCfg{}
	}

	n.BuildCfg.Online = true
	if n.BuildCfg.Repo == nil {
		n.BuildCfg.NilRepo = true
	}
	if n.BuildCfg.ExtraOpts == nil {
		n.BuildCfg.ExtraOpts = make(map[string]bool)
	}
	if n.BuildCfg.Host == nil {
		n.BuildCfg.Host = icore.DefaultHostOption
	}
	if n.BuildCfg.Routing == nil {
		n.BuildCfg.Routing = icore.DHTOption
		if n.LightClient {
			n.BuildCfg.Routing = icore.DHTClientOption
		}
	}
	if n.LogEntry == nil {
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		n.LogEntry = logrus.NewEntry(logger)
	}
}

// FillMandatory sets the mandatory fields.
func (n *NodeConfig) FillMandatory() {
	n.BuildCfg.ExtraOpts["pubsub"] = true
}
