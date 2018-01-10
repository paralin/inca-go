# Inca Go

> Light-weight proof-of-stake BFT consensus.

## Introduction

Inca is an attempt to implement an ultra-lightweight Proof-Of-Stake blockchain.

This repo contains the Go implementation. See [the root repo](https://github.com/aperturerobotics/inca).

## Inca CLI

This repository contains an implementation of the `inca` binary. It:

 - Can be used to build genesis blocks 
 - Can be used as a non-voting node to backup the blockchain in the local IPFS node
 
Building a blockchain participant requires embedding Inca in your own code.
