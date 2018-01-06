package shell

import (
	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/urfave/cli"

	sh "github.com/ipfs/go-ipfs-api"
)

// ShellFlags are the flags we append for setting shell connection arguments.
var ShellFlags []cli.Flag

var cliShellArgs = struct {
	// IpfsApi is the IPFS API endpoint.
	IpfsApi string
}{
	IpfsApi: "http://127.0.0.1:5001",
}

func init() {
	ShellFlags = append(ShellFlags, cli.StringFlag{
		Name:        "ipfs-api",
		Usage:       "IPFS API endpoint or path to use.",
		EnvVar:      "IPFS_API",
		Value:       cliShellArgs.IpfsApi,
		Destination: &cliShellArgs.IpfsApi,
	})
}

// BuildCliShell builds the shell from CLI args.
func BuildCliShell(log *logrus.Entry) (*sh.Shell, error) {
	s := sh.NewShell(cliShellArgs.IpfsApi)
	ver, commit, err := s.Version()
	if err != nil {
		return nil, errors.WithMessage(err, "cannot contact ipfs")
	}

	log.
		WithField("url", cliShellArgs.IpfsApi).
		WithField("version", ver).
		WithField("commit", commit).
		Info("contacted IPFS")
	return s, nil
}
