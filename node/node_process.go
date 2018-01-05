package node

import (
	"github.com/jbenet/goprocess"
)

// nodeProcess is the process managing the node.
func (n *Node) nodeProcess(process goprocess.Process) {
	process.AddChild(n.IpfsNode.Process())

	select {
	case <-n.Context().Done():
		return
	case <-process.Closing():
		return
	}
}
