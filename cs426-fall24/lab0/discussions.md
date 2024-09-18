1. An unbuffered channel is a channel with capacity of 1, which means that when a message is sent through that channel,
the channel blocks until a receiver consumes this value. On the other hand, a buffered channel if defined by its buffer
length, and blocks when the number of messages exceeds that capacity.

2. The default channel in Go is unbuffered.

3. The function defines an unbuffered channel that waits for a string value. The channel receives the string "hello world!".
However, because it is an unbuffered channel and there is no corresponding receiver at that moment, the program will block
on that send operation, waiting for a receiver to read from the channel. However, the receiving `<-ch` occurs after the send operation, so the program never reaches that point. This causes a deadlock because the program is stuck waiting for a receiver.

The channel receives the string 
"hello world!". Then the string is read from the channel and is stored in the 'message' variable.
Finally, "hello world!" is printed.

6. `<-chan T` defines a read-only channel of type T, i.e. you can only reveive data from the channel;
`chan<- T` defines a write-only channel of type T, i.e. you can only send data to the channel; 
`chan T` defines a bidirectional (read-write) channel of type T (both send and reveive channel),
i.e. you can send and receive data to and from the channel.

7. Reading from a closed channel will return the zero value of its type or any remaining data in the channel.
Reading from a nil channel will cause an indefinite block.

8. The for loop will terminate when the channel has closed and all of its elements have been read. If the channel
does not close, the loop will continue even if the channel does not have any elements to be read.

9. You can determine if a `context.Context` is done by checking the `Done()` channel; if the channel is closed,
the context is done, otherwise it is not. <br>
You can detetmine if `context.Context` is canceled by checking `Err()` function; if it was canceled, then
the error returned by `Err()` would be "context canceled".

10. The program will most likely print the following: <br>
```
all done!
1
2
3
```
This is because the gorroutines run concurrently. Inside each goroutine there is a delay of some time before printing
the value of i. However, at that time, the for loop will have terminated and, thus, "all done!", will be printed first.
Then, after the time delay of each goroutine finishes, the numbers will be printed.

10. To fix the issue in 8. and print "all done!" after the numbers, we can use synchronization primitives.
More specifically, we can use `sync.WaitGroup` and wait for the 3 goroutines to finish before accessing
`fmt.Println("all done!")`. Below are the changes to the initial code that need to be made to fix the issue.
```
var wg sync.WaitGroup
wg.Add(3)
for i := 1; i <= 3; i++ {
    go func() {
        defer wg.Done()
        time.Sleep(time.Duration(i) * time.Second)
        fmt.Printf("%d\n", i)
    }()
}

go func() {
	wg.Wait()
    fmt.Println("all done!")
}()
```
10. A mutex, `sync.Mutex`, allows a single goroutine to (exclusively) access a critical section at a time.
A semaphore, `semaphore.Weighted`, on the other hand, allows up to N goroutines to access a resource simultaneously 
rather than limiting the access to only a single goroutine.

11. The program prints the following:<br>
```
    []  
    0  
    true  

    0  
    <nil>
    {}
```
13. `struct{}` is a zero-size struct type (no information stored), often used when we need to signal events without
sending data. Using `chan struct{}` makes it clear that the channel is used solely for signaling, as values sent over
the channel have no content and take up zero bytes of memory.

14. The program will most likely print the following: <br>
```
4
4
4
all done!
```
This is because the goroutines run concurrently to the for loop. Inside each goroutine, there is a delay of some time
before printing the value of i. However, at that time, the for loop will have incremented the value of i to 4. Once,
the timer is over for each subroutine, the value of i will be printed, which will be the number 4.
