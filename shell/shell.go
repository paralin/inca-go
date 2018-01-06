package shell

import (
	iobjt "github.com/aperturerobotics/inca-go/objtable"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/pbobject/ipfs"
	api "github.com/ipfs/go-ipfs-api"
)

// Shell wraps the IPFS API with additional features.
type Shell struct {
	*ipfs.FileShell
	*iobjt.ObjectTable
}

// Wrap wraps the API shell.
func Wrap(sh *api.Shell) *Shell {
	ot := iobjt.NewObjectTable()

	return &Shell{
		ObjectTable: ot,
		FileShell:   ipfs.NewFileShell(sh, ot.ObjectTable),
	}
}
