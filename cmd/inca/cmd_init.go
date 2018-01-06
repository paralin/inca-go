package main

import (
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/timestamp"

	"github.com/golang/protobuf/jsonpb"
	"github.com/jbenet/goprocess"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

func init() {
	incaCommands = append(incaCommands, cli.Command{
		Name:   "init",
		Usage:  "initialize a blockchain by making the genesis block",
		Action: buildProcessAction(cmdInitCluster),
	})
}

func cmdInitCluster(p goprocess.Process) error {
	le := logctx.GetLogEntry(rootContext)
	sh, err := GetShell()
	if err != nil {
		return err
	}

	genesisTs := timestamp.Now()
	gobj := &inca.Genesis{
		ChainId:   "test-chain-1",
		Timestamp: &genesisTs,
	}

	tag, err := sh.AddProtobufObject(rootContext, gobj)
	if err != nil {
		return err
	}

	le.WithField("hash", tag).Info("successfully built genesis block")

	le.Info("confirming block")
	obj, err := sh.GetProtobufObject(rootContext, tag)
	if err != nil {
		return err
	}

	gout, ok := obj.(*inca.Genesis)
	if !ok {
		return errors.Errorf("unexpected decrypted object: %#v", obj)
	}

	jstr, err := (&jsonpb.Marshaler{}).MarshalToString(gout)
	if err != nil {
		return err
	}
	le.Infof("retrieved genesis block: %s", string(jstr))

	return nil
}
