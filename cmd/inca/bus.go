package main

import (
	"errors"
	"sync"

	"github.com/aperturerobotics/controllerbus/bus"
	"github.com/aperturerobotics/hydra/core"
	"github.com/aperturerobotics/hydra/volume"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/urfave/cli"
)

var coreBusMtx sync.Mutex
var coreBusCached bus.Bus
var coreBusBucketOp = &volume.BucketOpArgs{}

func init() {
	incaFlags = append(
		incaFlags,
		cli.StringFlag{
			Name:        "volume-id",
			Usage:       "the hydra volume id to use",
			Destination: &coreBusBucketOp.VolumeId,
		},
		cli.StringFlag{
			Name:        "bucket-id",
			Usage:       "the hydra bucket id to use",
			Destination: &coreBusBucketOp.BucketId,
		},
	)
}

func GetCoreBus() (bus.Bus, *volume.BucketOpArgs, error) {
	if coreBusBucketOp.GetBucketId() == "" ||
		coreBusBucketOp.GetVolumeId() == "" {
		return nil, nil, errors.New("volume and bucket id must be specified")
	}

	coreBusMtx.Lock()
	defer coreBusMtx.Unlock()

	if coreBusCached != nil {
		return coreBusCached, coreBusBucketOp, nil
	}

	b, sr, err := core.NewCoreBus(
		rootContext,
		logctx.GetLogEntry(rootContext),
	)
	if err != nil {
		return nil, nil, err
	}
	_ = sr

	coreBusCached = b
	return coreBusCached, coreBusBucketOp, nil
}
