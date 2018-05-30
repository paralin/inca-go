# Mempool Proposer Implementation

This module enables Bitcoin-style In-Memory Transaction Pooling.

 - The state, and transactions against the state, are opaque interfaces to this module.
 - Transactions are sent over the Inca NodeMessage channel, with any opaque storage reference'd transaction body.
 - Transaction submission time and author are inferred from the containing NodeMessage.
 - All internal decision making components are modularized.
 
Important components:

 - Transaction: opaque storage reference to any measurable change to state.
 - Mempool: pool of transactions that are waiting to be applied to the state.
 - WorkingSet: work-in-progress proposal for the next state, converts into a block.
 - NodeMessage: contains a reference to the transaction object.

## WorkingSet/Block Proposers

This section discusses available strategies for building the WorkingSet.

### FIFOProposer

Builds the proposal over time as transactions arrive.

  - If a peer makes a proposal, work is useless.
  - Only suitable for single-proposer networks, but this behavior is the same as OnDemandProcessor.
  - Not implementing this at this time.
  
### OnDemandProposer

Builds the proposal on-demand, usually when the round begins.
 
 - Consider transactions in order.
 - Preempt the operation when the working set reaches a certain size, or a time limit is reached.
 - Expired transactions are never considered (skip).
 - Computation can be pre-empted when the proposal reaches a certain size, or time constraint.
 - Can wait until enough transactions have arrived before making a proposal.
 - Downside: proposal must be built immediately when new round begins (no pre-computation).
 
**The OnDemandProposer will be the first implementation.**

### How should the mempool sort transactions and keep them sorted?

We effectively need an iterator for the pool to express any strategy of:

 - Return list of keys, sort by parameter, pick via that parameter.
 - Scan keys (ex. redis), insert into sorted set (priority queue), pick from sorted set.
 - Maintain sorted set in storage (heap) and pick from that set.
 
Exploring the heap idea some more, with the underlying store as k/v:

 - Need a better data structure: Concurrent, Sorted, K/V Backing
 
Effective structure for an iterator might look like:

 - Peek: show the current selected transaction at the top of the heap.
 - Pop: pop the selected transaction off the top of the heap, marking it as included

There are a few considerations here:

 - Backing DBs like Redis can return a stream of keys (SCAN).
 
The iterator must consider that some keys may be removed from the backing store. This could be used as a sort-of "gateway" to the backing storage.

Example algorithm for efficient top-of-stack priority queuing:
 
Data structure described:

 - Bounded-sized: only N elements held in memory at a time.
 - Sorted: by a stable parameter, can be used as a priority queue.
 - Storage-backed: optimal for large mempools with a fast priority factor.
 - Sweeping: must sweep the entire pool when clearing the priority queue or removing anything from it.

Answer: use a fibbonachi heap! It does all these things.
 
## Sorted Mempool: Fibbonaci Heap

Stats:

 - Find-Minimum: O(1)
 - Insert: O(1)
 - Delete element: O(log(n))
 - Merge Heaps: O(1)
 
This seems like a good candidate for mempool sorting.

Other notes:

 - Fast insert, good for time-sorted mempool.
 
Implemented in objstore.

## Mempool Proposer

When a transaction is received:

 - A NodeMessage arrives with a App message type with an inner type as Transaction.
 - The transaction node message and author is briefly validated against some simple flooding and admission rules.
 - The transaction is enqueued in a priority queue sorted by message time (TX queue) of un-checked tx.
 - The application PrePoolCheckTx routine is executed, which might fetch the transaction body and verify it.
 - The transaction is promoted to the "hot" TX queue.
 - A transaction is selected for inclusion in the next proposal, and the application CheckTx, then ApplyTx calls are made.
 - Eventually, the proposal is complete, and submitted to the network.
 
Building the proposal involves:

 - Until the proposal deadline, collect transactions with dequeue-min.
 
## Mempool Pipeline Proposal Validator

Validating an incoming proposal involves a buffered pipeline of included transactions:

 - txverify: ensure the transaction is in the mempool, and valid.
 - txapply: apply the transaction to the current working state.
 - txcompare: check that the new state matches the proposed new state.
 
After validating the new state, the mempool is "flushed":

 - All transactions that were just applied are removed from the pool.
 - All expired transactions are removed from the pool.
 - The "working state" is reset to the latest applied state.
 - All transactions that were not just applied are marked as unchecked.

The "working state" structure will contain:

 - Current HEAD of the "working state"
 - Fibbonaci heap of applied transaction IDs.
 
Mempool flushing is periodically triggered.

## Transaction IDs

Transaction IDs are the sha256 string of:

 - Submitter ID
 - Submission timestamp
 
Both values are derived from the node message.
 
## Proposal/Block Structure

The Inca BlockHeader contains:

 - Reference to genesis block
 - Chain configuration reference
 - Proposed next chain configuration reference
 - Last block reference
 - Round information
 - Block timestamp
 - Proposer identifier
 - State reference, opaque pointer.
 
Storage references can contain "InBand" data to append to the block rather than point to the data.

This is leveraged in the Mempool proposer, to store additional data in the block (in the state ref):

 - Transaction set reference
 - Application state reference

The transaction set contains the ordered list of transactions processed in the block.
The application state is an opaque pointer.


