package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
)

var dbMtx sync.Mutex
var dbCached db.Db

func init() {
	incaFlags = append(incaFlags, db.DbFlags...)
}

// GetDb builds / returns the db.
func GetDb() (db.Db, error) {
	dbMtx.Lock()
	defer dbMtx.Unlock()

	if dbCached != nil {
		return dbCached, nil
	}

	le := logctx.GetLogEntry(rootContext)
	d, err := db.BuildCliDb(le)
	if err != nil {
		return nil, err
	}

	dbCached = d
	return dbCached, nil
}
