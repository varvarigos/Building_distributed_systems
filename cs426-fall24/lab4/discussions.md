### A4

My TTL strategy spawns a goroutine for each shard (i.e. O(|S|) spawns, where |S| is the number of shards), running a ticker that checks
all keys in the shard and deletes expired ones (with a time complexity of O(|K|), where |K| is the number of keys in the shard). This approach
ensures the timely removal of stale data, preventing excessive memory usage by regularly clearing expired keys. However, it introduces a consistent
processing cost due to the frequent locking of each shard's data during the deletion process, which can become expensive as the number of keys per
shard grows.

In a write-intensive scenario, this approach may lead to contention between cleanup and write operations, potentially impacting performance. An
alternative strategy could involve checking each entry for expiration only when it is accessed, reducing the need for constant background
cleanup. While this may allow expired keys to be available in the shard for longer, it would make the system more efficient overall in a
write-heavy environment. Another option could involve a background batch cleanup that periodically scans all shards in a staggered manner, minimizing
the number of active goroutines and reducing thus the locking frequency on shard data. This would increase the time required to delete expired keys but
might be preferable if `Get` requests are significantly less frequent than `Set` requests (which is the case in a write-intensive environment).
Another way to increase efficiency using my current implementation without requiring a lot of changes is to reduce the ticker frequency, which
would increase the time between the next expired-key deletions, i.e. the next locking of the shard data, and would result in less frequent cleanup
operations, but again would be preferable in a write-intensive scenario.

### B2

The current load balancing strategy uses a random index offset to distribute `Get` requests across available nodes. By generating a single random offset for
each `Get` request, we can start each request at a different node, providing a generally uniform distribution over time. However, this approach has
limitations that could lead to load imbalances. Since the randomness relies on a single offset per request, it does not guarantee even distribution across all nodes
in short intervals. This can result in some nodes handling significantly more requests than others, especially if the random offset happens to favor a subset of
nodes repeatedly, and some other nodes handling very few requests and become underutilized. Additionally, this stateless approach does not account for differences in node
capacity, such as CPU resources or the number of shards already assigned, which can lead to some nodes being overloaded while others remaining underutilized. The lack of
session persistence is another issue, as subsequent requests from the same client are not guaranteed to be routed to the same nodes, which reduces cache effectiveness
and affects the performance of the system.

Several alternative strategies could help mitigate these issues. A weighted round-robin approach could account for differences in node capacity, assigning a larger
proportion of requests to nodes with greater CPU resources or fewer shards, thus reducing the load on overutilized nodes. Consistent hashing could also be used to
assign each key to specific nodes in a balanced way that minimizes reassignments when nodes are added or removed. For a more dynamic approach, a load balancer
that monitors real-time metrics such as CPU load or request queue length would be able to distribute requests in a way to maintain balance.

### B3

The current failover strategy sequentially attempts `Get` requests across all available nodes until one succeeds. This, however, can lead to higher waiting times,
especially if multiple nodes are slow or unresponsive since requests are retried across all hosts. To address this issue, introducing request timeouts for each node
could prevent delays from slow responses. Additionally, implementing a simple tracking system for node responsiveness could prioritize nodes with recent success rates,
enhancing both request efficiency and reliability by reducing unnecessary retries on less responsive nodes.

### B4

In the case of partial failures of `Set` calls, some nodes hosting the shard successfully process the `Set` request, while others fail. This can result in inconsistencies across
nodes, where only a subset reflects the latest value for a key. Therefore, a client running a subsequent `Get` request might observe stale data if they happen to query
a node that failed the previous `Set` call. Moreover, a client might even observe different values when querying for the same key depending on the node it requests the
data from, thus potentially leading to data inconsistency and non-deterministic behavior.

### D2

#### Experiment 1

This experiment tested the system's ability to handle a high load of both Get and Set requests (1000 QPS each) on a 5-node configuration.

```
go run cmd/stress/tester.go --shardmap shardmaps/test-5-node.json --get-qps 1000 --set-qps 1000 --duration 1m

Stress test completed!
Get requests: 59679/59679 succeeded = 100.000000% success rate
Set requests: 59646/59646 succeeded = 100.000000% success rate
Correct responses: 59532/59532 = 100.000000%
Total requests: 119325 = 1988.670931 QPS
```

The system performed reliably at high load, achieving 100% success across all requests without errors or drops, indicating efficient load
distribution under heavy traffic.

#### Experiment 2

This test aimed to assess performance with a high shard count (100 shards) at a moderate request rate of 250 QPS for both Get and Set.

```
go run cmd/stress/tester.go --shardmap shardmaps/test-3-node-100-shard.json --get-qps 250 --set-qps 250 --duration 1m

Stress test completed!
Get requests: 15020/15020 succeeded = 100.000000% success rate
Set requests: 15020/15020 succeeded = 100.000000% success rate
Correct responses: 15004/15004 = 100.000000%
Total requests: 30040 = 500.658264 QPS
```

The system maintained 100% reliability and accuracy. The moderate QPS and high shard count showed that the nodes distributed load effectively
even with a high number of shards on a smaller node set, indicating that the system scales reasonably well with increased shard numbers.

#### Experiment 3

This experiment testing how the system would handle a larger disparity between Set and Get QPS, with Set requests set four times higher than
Get requests to test write-intensive load.

```
go run cmd/stress/tester.go --shardmap shardmaps/test-5-node.json --get-qps 250 --set-qps 1000 --duration 1m

Stress test completed!
Get requests: 15020/15020 succeeded = 100.000000% success rate
Set requests: 60010/60010 succeeded = 100.000000% success rate
Correct responses: 14975/14975 = 100.000000%
Total requests: 75030 = 1250.476183 QPS
```

The system successfully managed the increased write load, maintaining 100% success and accuracy, indicating resilience under write-intensive scenarios.

#### Experiment 4

This experiment inverted the request rates from Experiment 3, focusing on read-intensive load with a higher Get QPS relative to Set.
    
```
go run cmd/stress/tester.go --shardmap shardmaps/test-5-node.json --get-qps 1000 --set-qps 250 --duration 1m

Stress test completed!
Get requests: 59967/59967 succeeded = 100.000000% success rate
Set requests: 15020/15020 succeeded = 100.000000% success rate
Correct responses: 59924/59924 = 100.000000%
Total requests: 74987 = 1249.752573 QPS
```

The system handled the read-heavy load effectively, maintaining 100% success for both Get and Set operations, highlighting consistent
performance even under varying read/write demands.

#### Experiment 5

This test assessed the system's ability to handle dynamic changes in the shard map. While the experiment was running, I updated
the shard map to simulate migrations: moving, adding, and removing mutliple nodes from shards to test the shard migration logic.

```
go run cmd/stress/tester.go --shardmap shardmaps/test-3-node.json

Stress test completed!
Get requests: 6020/6020 succeeded = 100.000000% success rate
Set requests: 1820/1820 succeeded = 100.000000% success rate
Correct responses: 6020/6020 = 100.000000%
Total requests: 7840 = 130.663387 QPS
```

The shardmap watcher successfully detected and propagated changes, and the servers adjusted without disruption. This demonstrated the reliability
of the shard migration implementation and confirmed that clients could maintain accurate routing in dynamically changing shard maps.