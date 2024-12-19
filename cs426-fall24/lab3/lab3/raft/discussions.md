# 3A-2

## (1) Construct and describe a scenario with a Raft cluster of 3 or 5 nodes where the leader election protocol
fails to elect a leader. Hint: in your description, you may decide when timers time out or not time out, or
arbitrate when RPCs get sent or processed.

Consider a Raft cluster of 3 nodes. Initially, each node sets its election timeout independently, but these
timeouts are close to each other due to the randomization of timeouts being within a similar range.

- Node A's election timeout triggers first, so it becomes a candidate, increments its term, and sends
`RequestVote` RPCs to nodes B and C.

- Before nodes B and C can process Node A's `RequestVote`, their own election timeouts also trigger. Nodes B
and C transition to the candidate state, increment their terms (higher than Node A's term now), and each sends
out its own `RequestVote` RPCs to the other nodes.

- Since all nodes have become candidates in rapid succession: Node A votes for itself; Node B votes for itself
and rejects Node A's `RequestVote` because its term is now higher; Node C votes for itself and similarly rejects
Node A's and B's requests due to the higher term. Thus, none of the nodes can gather a majority (2 out of 3 votes),
leading to a failed election round.

- The process repeats as the nodes' election timeouts expire again in roughly the same order, with each node
restarting the election without ever gaining a majority. Therefore, no leader is elected because the randomized
election timeouts are too close, and the `RequestVote` RPCs either do not arrive in time or are rejected by
nodes already in a higher term, leading to indefinite election cycles without progress.

This results in the leader election protocol failing to elect a leader.

## (2) In practice, why is this not a major concern? i.e., how does Raft get around this theoretical possibility?

In practice, Raft mitigates this issue through randomized election timeouts. Raft introduces random delays in
election timeouts to reduce the likelihood that multiple nodes start elections simultaneously. This randomness
makes it unlikely that all nodes will start an election at the same time, thus increasing the probability of one
node gaining the majority.

Additionally, once a leader is elected, the leader sends regular heartbeats to reset the election timeouts of
the followers, preventing them from initiating an election unless the leader fails or becomes partitioned.

# ExtraCredit1

The scenario described above could be resolved by implementing Pre-Vote and CheckQuorum mechanisms, which help
prevent unnecessary term increases and split-vote scenarios.

In the case where nodes A, B, and C trigger their election timeouts simultaneously, Pre-Vote would prevent them
from starting an election without first checking if they can win. Since nodes B and C would not grant pre-votes
to node A (due to likely upcoming timeouts), node A would not proceed to candidate status, preventing the
simultaneous elections and term inflation. Similarly, B and C would also not proceed if they cannot gather
pre-votes. Also, CheckQuorum ensures that only a node with the majority of votes can remain a leader, preventing
split votes and endless election cycles.

Even with Pre-Vote and CheckQuorum, Raft can still encounter situations where leader election gets stuck.
For instance, in a 5-node cluster, a network partition might split the nodes into two groups—one with three
nodes and the other with two. In this case, both groups might attempt elections: the three-node group could
initiate a leadership election using Pre-Vote, and the two-node group could do the same. However, neither
group holds a majority of the cluster's total nodes, meaning CheckQuorum would prevent either side from
establishing a leader. This deadlock persists until the network partition is resolved, without Pre-Vote
and CheckQuorum being able to resolve the issue.
