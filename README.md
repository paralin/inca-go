# Inca Go

> Light-weight proof-of-stake BFT consensus.

## Introduction

Inca is an attempt to implement an ultra-lightweight Proof-Of-Stake blockchain.

This repo contains the Go implementation. See [the root repo](https://github.com/aperturerobotics/inca).

## Inca CLI

This repository contains an implementation of the `inca` binary. It:

 - Can be used to build genesis blocks 
 - Can be used as a "light" node to backup the chain traffic in the local IPFS node
 - Can be used as a full validator node.
 
Building a blockchain participant requires embedding Inca in your own code.

## IPFS Notes

Currently there are the following known requirements:

 - Run ipfs with `--enable-pubsub-experiment`
 - Configure IPFS with "Routing.Type" to "none" to disable routing.
