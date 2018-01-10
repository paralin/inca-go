package db

import (
	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/objtable"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"os"
)

// DbFlags are the flags we append for setting shell connection arguments.
var DbFlags []cli.Flag

var cliDbArgs = struct {
	// DbType is the DB type to use.
	// badgerdb is the only supported value
	DbType string
	// DbPath is the path to store data in.
	DbPath string
}{
	DbType: "badgerdb",
	DbPath: "./data",
}

func init() {
	DbFlags = append(
		DbFlags,
		cli.StringFlag{
			Name:        "db-type",
			Usage:       "The DB type to use, badgerdb is the only supported value.",
			EnvVar:      "DB_TYPE",
			Value:       cliDbArgs.DbType,
			Destination: &cliDbArgs.DbType,
		},
		cli.StringFlag{
			Name:        "db-path",
			Usage:       "The path to store data in.",
			EnvVar:      "DB_PATH",
			Value:       cliDbArgs.DbPath,
			Destination: &cliDbArgs.DbPath,
		},
	)
}

// BuildCliDb builds the db from CLI args.
func BuildCliDb(log *logrus.Entry, objectTable *objtable.ObjectTable) (Db, error) {
	if cliDbArgs.DbType != "badgerdb" {
		return nil, errors.Errorf("unsupported db type: %s", cliDbArgs.DbType)
	}

	if err := os.MkdirAll(cliDbArgs.DbPath, 0755); err != nil {
		return nil, err
	}

	badgerOpts := badger.DefaultOptions
	badgerOpts.Dir = cliDbArgs.DbPath
	badgerOpts.ValueDir = cliDbArgs.DbPath
	bdb, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, err
	}

	return NewBadgerDB(bdb, objectTable), nil
}
