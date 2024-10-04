## A1 Discussion

`grpc.NewClient()` is a function that creates a new client object used to send requests to a server.
It abstracts traditional socket API calls such as `socket()`, `connect()`, `send()`, and
`recv()` into a high-level API that simplifies the creation of a communication channel between a client
and a server. It handles the process of opening a connection (`socket()`) to the server, establishing the
connection (`connect()`), sending requests (`send()`) to the server, and receiving responses (`recv()`)
from the server. The client object is also responsible for managing the connection to the server and
handling any errors that occur during the communication process.

## A2 Discussion

`grpc.NewClient()` can fail in several scenarios: For network issues or when the server is unreachable,
`codes.Unavailable` should be returned. If the TLS authentication fails, then `codes.Unauthenticated` should be
returned. In cases of resource exhaustion, such as too many open connections, `codes.ResourceExhausted` should be
returned. If the connection establishment or request processing exceeds some specified timeout duration, then
`codes.DeadlineExceeded` should be returned. If the server address is incorrect, `codes.InvalidArgument` should be
returned. If the failure stems from internal gRPC client errors, returning `codes.Internal` should be returned.


## A3 Discussion

When calling `GetUser()`, networking/system calls like `socket()`, `connect()`, `send()`, and `recv()`
will be used under the hood to open and establish the connection and transmit data over the network.
They can return an error due to network issues (e.g., server unreachable, timeout), server-side issues
(e.g., internal errors), or resource exhaustion (e.g. too many open connections). Additionally, `GetUser()`
can return logical errors even when the network calls succeed, such as when no user is found (`codes.NotFound`)
or when the user does not have the necessary permissions (`codes.PermissionDenied`).

Errors are detected by checking the return value (result, err) of `GetUser()`. If err is not nil, gRPC
error codes (e.g., `codes.Unavailable`, `codes.NotFound`) are used to identify the type of error.

## A4 Discussion

If we use the same `conn` for both `UserService` and `VideoService`, it would result in an error because the
`conn` for `UserService` is dialed to the user server address and cannot be used to communicate with the
video server, which has a different address (and vice versa). Each service requires its own connection
to the correct server address.

## A6 Discussion

The name of the user is Pouros1409 and their top recommended video is titled "queer Dandelion Greens"
(with video id 1139 and url https://video-data.localhost/blob/1139). The full breakdown of the user's
recommended videos is as follows:

```
2024/09/24 23:00:57 Welcome av774! The UserId we picked for you is 200998.

2024/09/24 23:00:57 This user has name Pouros1409, their email is davinbeer@klein.info, and their profile URL is https://user-service.localhost/profile/200998
2024/09/24 23:00:57 Recommended videos:
2024/09/24 23:00:57   [0] Video id=1139, title="queer Dandelion Greens", author=Tania Casper, url=https://video-data.localhost/blob/1139
2024/09/24 23:00:57   [1] Video id=1212, title="Foxride: unlock", author=Estella Emmerich, url=https://video-data.localhost/blob/1212
2024/09/24 23:00:57   [2] Video id=1367, title="enthusiastic Onion", author=Gustave Marquardt, url=https://video-data.localhost/blob/1367
2024/09/24 23:00:57   [3] Video id=1013, title="fancy Bitter Melon*", author=Bettye Harris, url=https://video-data.localhost/blob/1013
2024/09/24 23:00:57   [4] Video id=1138, title="clumsy upstairs", author=Peyton Bogan, url=https://video-data.localhost/blob/1138
2024/09/24 23:00:57
```

## A8 Discussion

We do not send batched requests concurrently in our implementation, thus minimizing the risk of potential
race conditions while also simplifying our design. However, one downside is the increased latency, as we
must wait for each batch to complete before sending the next. While introducing concurrency could reduce
the total waiting time, it would also add complexity in managing multiple concurrent requests and increase
the memory usage requirements. Additionally, sending too many concurrent requests could risk overwhelming
the server and result in higher latency or failures.

To reduce the total number of requests to UserService and VideoService using batching, the following steps
can be taken: <br>
1. Combine multiple incoming requests within a short time window, removing any duplicate IDs.
2. Split the combined request IDs into smaller groups, ensuring each group respects the maxBatchSize limit.
3. Process these batches concurrently to reduce latency and improve efficiency.
4. Once batch responses are received, map them back to the original requests.

```
// Step 1
while timer is active:
    receive request
    add userID/videoID to requestQueue (skip if duplicate)

// Step 2
for each ID in requestQueue:
    add ID to currentBatch
    if currentBatch.size == maxBatchSize:
        add currentBatch to batches
        reset currentBatch

// Step 3
for each batch in batches:
    run goroutine to send batch to the server
    store batch response

// Step 4
for each incoming request:
    get response and send it back
```

## B2 Discussion

`go run cmd/stats/stats.go`:

```
total_sent:19   total_responses:19      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:109.00
total_sent:30   total_responses:29      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.34
total_sent:39   total_responses:39      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:85.87
total_sent:49   total_responses:48      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:90.54
total_sent:59   total_responses:59      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.34
total_sent:70   total_responses:68      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:91.32
total_sent:80   total_responses:79      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:87.84
total_sent:89   total_responses:88      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.44
total_sent:99   total_responses:99      total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.49
total_sent:109  total_responses:108     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:90.41
total_sent:119  total_responses:119     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.47
total_sent:129  total_responses:129     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:89.71
total_sent:140  total_responses:139     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:90.12
total_sent:149  total_responses:148     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:91.86
total_sent:160  total_responses:158     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:92.80
total_sent:169  total_responses:169     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:92.83
total_sent:180  total_responses:179     total_errors:0  failure_rate:0.00%      stale_responses:0       avg_latency_ms:90.38
```

`go run cmd/loadgen/loadgen.go --target-qps=10`:

```
now_us  total_requests  total_errors    active_requests user_service_errors     video_service_errors    average_latency_msp99_latency_ms  stale_responses
1727224056509300        309     0       0       0       0       89.43   0.00    0
1727224057512355        309     0       0       0       0       89.43   0.00    0
1727224058514494        309     0       0       0       0       89.43   0.00    0
1727224059514978        309     0       0       0       0       89.43   0.00    0
1727224060516797        309     0       0       0       0       89.43   0.00    0
1727224061511451        309     0       0       0       0       89.43   0.00    0
1727224062510843        322     0       0       0       0       91.11   0.00    0
1727224063511582        332     0       0       0       0       90.20   0.00    0
1727224064510840        342     0       1       0       0       88.99   0.00    0
1727224065510970        352     0       1       0       0       88.80   0.00    0
1727224066510592        362     0       1       0       0       89.44   0.00    0
1727224067511177        372     0       1       0       0       89.20   0.00    0
1727224068511065        382     0       0       0       0       89.57   0.00    0
1727224069511600        392     0       1       0       0       88.86   0.00    0
1727224070510353        402     0       1       0       0       89.26   0.00    0
1727224071511849        412     0       1       0       0       89.30   0.00    0
1727224072512497        422     0       1       0       0       89.50   0.00    0
```

## C1 Discussion

If the root cause of the failure persists, retries can waste valuable resources (e.g. network bandwidth,
CPU utilization), and may overload the target server. Also, retries can introduce unnecessary delays,
particularly when the failure is not transient. If the retries continue to fail, they can cause increase
of latency and resource consumption without resolving the issue. Additionally, in cases where the
request modifies data, multiple retries can lead to inconsistent states.

Retries should not be attempted in cases of non-recoverable errors (e.g. service outages, configuration
errors, invalid requests). Furthermore, retries are risky for operations that modify data, as they may
lead to inconsistent states if the same operation is executed multiple times. Retrying should also be
avoided for requests with strict deadlines, as it can cause unnecessary delays and waste resources without
addressing the core issue.

## C2 Discussion

If the `VideoService` is still down and the cached videos have expired, returning expired responses is
preferred over returning an error. Therefore, the user will still receive some recommendations, even if
they are not the most up-to-date. The tradeoff is that users will receive potentially stale content, which
may not reflect current trends. The alternative of returning an error can be disruptive, but the benefit would
be that it explicitly signals when the content is unreliable. However, it is generally better to offer some
form of fallback rather than failing the request entirely, as this maintains service availability and a better
user experience.

## C3 Discussion

One strategy would be to monitor the number of failed requests to UserService or VideoService. If the number of
failures reaches a certain threshold, the system would stop making further requests to the service for a short
period and rely on fallback options. This would avoid repeated failures and significant performance degradation,
but it could potentially delay the access to up-to-date data once the service recovers, in cases where this timeout
is too long.

Another strategy could be to cache a set of previously recommended videos for each user and return this cache instead
of the trending videos during service failures, which would result in more useful and personalized recommendations to
the users. However, this would increase the memory consumption since caching would need to be maintained for each user.
Additionally, cached recommendations might also become stale as user preferences change, but they would still return
more relevant content than trending videos.

## C4 Discussion

Establishing new connections via `grpc.NewClient()` for each request is costly due to the overhead of DNS resolution,
TCP handshakes, and SSL/TLS negotiation. For a high-throughput service, this can lead to increased latency, reduced
throughput, and resource exhaustion.

To mitigate this, a connection pool can be used. That is, instead of creating a new connection for every request, the
service would reuse existing connections for multiple requests. This will reduce the overhead and latency by
establishing the connection only once.

Using a connection pool introduces some trade-offs. Firstly, maintaining a connection pool increases memory usage, as
these connections remain active even when not in use. Another potential issue is reusing stale or failed connections,
leading to errors if the pool does not properly manage them. Additionally, there is connection management overhead,
especially in scenarios where the server frequently restarts or the network topology changes, which can lead to costly
re-establishment of connections.