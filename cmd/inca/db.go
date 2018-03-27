package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore/db"
	dbcli "github.com/aperturerobotics/objstore/db/cli"
)

var dbMtx sync.Mutex
var dbCached db.Db

func init() {
	incaFlags = append(incaFlags, dbcli.DbFlags...)
}

// GetDb builds / returns the db.
func GetDb() (db.Db, error) {
	dbMtx.Lock()
	defer dbMtx.Unlock()

	if dbCached != nil {
		return dbCached, nil
	}

	le := logctx.GetLogEntry(rootContext)
	d, err := dbcli.BuildCliDb(le)
	if err != nil {
		return nil, err
	}

	dbCached = d
	return dbCached, nil
}
