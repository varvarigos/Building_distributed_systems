### A3 - extra credit

A UUID (Universally Unique Identifier) is a 128-bit number used to uniquely identify information in systems,
with a near certainty that the same identifier has not and will not be generated for something else.

The generation of a UUID ensures uniqueness due to the extremely low probability of collision, given the 128-bit
size (approximately $3.4e+38$ possible values) and the methods used to generate it (depending on the version).
For example, version 1 UUIDs are based on the current timestamp (in nanoseconds) and the machine's MAC address,
while version 4 UUIDs are generated randomly, further reducing the likelihood of collisions to be negligible.

### B1

I believe that deleting the pod will cause Kubernetes to automatically create a new pod, maintaining the desired
number of replicas (in my case $1$). After running `kubectl delete pod <pod-name>`, the pod was deleted, and Kubernetes
indeed created a new pod.

### B4

```
Welcome! You have chosen user ID 202408 (Koss4583/heavenerdman@jacobs.biz)

Their recommended videos are:
 1. The sparkly swan's safety by Rhett Ledner
 2. The energetic raccoon's money by Laron Keeling
 3. panicked upstairs by Brendon Gorczany
 4. The thankful hamster's chaos by Alek Strosin
 5. The aloof raccoon's welfare by Luella Barrows
```

### C3

With a client pool size of 4, the load is spread more evenly across the two service hosts, showing 5 peaks,
which indicates better load distribution compared to using 1 client. If only 1 client is used, the traffic would
concentrate on a single host, i.e. a single peak. Increasing the pool size to 8 would lead to
even better load balancing, with the traffic being more uniformly distributed across the available hosts.