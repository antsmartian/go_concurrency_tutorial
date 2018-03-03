#### A minimal in-memory database

Learning Golang can be quite tricky when it comes to concurrency. I'm in the process of learning Golang
and found it bit hard to understand concurrency and locks. I created a simple in-memory database, that allow
you to store key-value pair of type `string`. Well all this is done for understanding mutex's and go's
concurrency. The code of the entire database would be around `100` lines! Simple & Elegant. Inspired from BoltDB source.

Note: The key-value pair is stored in `map`. I expect you all have a beginner level understanding of
golang syntax and primitive types to understand the source-code. There could be instances, where I'm wrong in explaining things,
please raise a PR for the same.  Thanks in advance :)

###### Defining the interfaces:

We are going to store a simple key-value pair in our memory database. The idea is that our database
is fully safe to use in concurrent environment, hence we would be using mutex to lock our data.
We will use `map` as our datastore, so lets go and define a simple struct that puts our requirement
on target:

```
type DB struct {
	mu sync.RWMutex
	data map[string]string
}
```

Note: In fact we could have used files to save our data, but I'm keeping it simple.

We will discuss why `mu` is declared as `sync.RWMutex` and understand its use case later on.

Now to declare and create our `DB`, lets go and create a simple function that does for us:

```
func Create() (*DB) {
	db := &DB{
		data : make(map[string]string),
	}
	return db
}
```

the above function does initialize our `data` to be of type `[string]string`.


####### Creating APIs.

We will be exposing our API's which will allow the end user to put their data into the memory. Since we are
calling this a in-memory database, we will create a struct which holds the current transaction data:

```
type Tx struct {
	db *DB
	writable bool
}
```

here, `db` is just a reference to our low lying database and `writable` is a bool type, which says whether
the transaction happening is for Reading or Writing.

Since all our operation of setting and getting the value is via a transaction, we will add functions to our
`Tx` struct:

```
func (tx *Tx) Set(key, value string) {
	tx.db.data[key] = value
}

func (tx *Tx) Get(key string) string {
	return tx.db.data[key]
}
```

Nothing interesting here, a simple `Set` and `Get` for our underlying datastore. Now here comes the interesting
part, since we are going to be concurrently safe, we need someway to **protect our data**. Computer science
has very old concept of **locks**, which will allow us to protect our data in concurrent env.
Well, Golang has a support for locks at the language level.

###### What are locks?

Simply put **locks protect a shared memory (once acquired), until the lock is released**. Here our shared memory is our
`map`. So how we are going to protect the following scenario:

```
T1 -> Set key,value1 => map
T1 -> Set key,value2 => map
T2 -> Get key <= map
```

Imagine at time T1, we are getting two request for `Set` to our `map`. Well, what will be the value of `key`
at T2, when you do a `Get`? Yes, its confusing and hard to say and there is no correct answer. If we run
the above pseudo code for 1000 times, we get different answer at T2 (either `value1` or `value2`). The reason
for this is the underlying datastore our `map` is **shared** between concurrent or parallel process.

In order to solve the issue, we need a lock!


####### Mutex In Golang

Remember our `DB` struct:

```
type DB struct {
	mu sync.RWMutex
	data map[string]string
}
```

the `mu` is of type `sync.RWMutex`. Actually speaking we could have used `Mutex` package, but we will use
`RWMutex` instead, which stands for `ReadWrite Mutex`. A mutex does provide a `lock` method which will
prevent accessing the shared memory until the corresponding `unlock` method is called. So the psudeocode,
looks like the following:

```
lock()
    //access map
unlock()
```

with our `lock` and `unlock` functions in place, our previous concurrent code would be safe:


```
T1 -> Set key,value1 => map //acquires the lock
T1 -> Set key,value2 => map //wait till the lock is released by first acquired lock
T2 -> Get key <= map  //wait till the lock is released by first acquired lock
```

As we could able to see, with `lock` we are protecting our data! Now that's about lock in simple terms,
what is so special about RWMutex? Well, in our simple in memory database, we want our caller of `Get`
not to wait i.e there could be concurrent reads without any blocks. Something like the below is totally
acceptable and scalable solution:

```
T1 -> Get key <= map
T1 -> Get key2 <= map
T1 -> Get key3 <= map
T1 -> Get key4 <= map
T1 -> Get key5 <= map
```

but there is a catch here. What happens when our callstack looks like the following:

```
T1 -> Get key <= map
T1 -> Get key2 <= map
T1 -> Get key3 <= map
T1 -> Get key4 <= map
T1 -> Set key4,value4 => map
T1 -> Get key5 <= map
```

oh oh, `Get`/`Set` call for `key4` is in **race condition** and our database won't be consistent!
What we could do? That's where RWMutex comes into picture! RWMutex has a special type of lock called
as RLock, which eventually means Read Lock. The way Read Lock works is as follows:

1. There could be n number of Read Locks (without blocking other Read Lock)
2. However, Write Lock can't be acquired until all Read Locks are released.

Example, a valid scenario:

```
RLock() <- Thread 1
RLock() <- Thread 2
RLock() <- Thread 3
RLock() <- Thread 4
//... read operations..
RUnlock() <- Thread 1
RUnlock() <- Thread 2
RUnlock() <- Thread 3
RUnlock() <- Thread 4
```

Completely fine and note that all are running concurrently. However the below scenario:

```
RLock() <- Thread 1
RLock() <- Thread 2
RLock() <- Thread 3
RLock() <- Thread 4
//... some operations..
WLock() <- Thread 5 //blocked until all Threads call RUnlock();  can't acquire the lock, waiting..
RUnlock() <- Thread 1
RUnlock() <- Thread 2
RUnlock() <- Thread 3
RUnlock() <- Thread 4
```


This is exactly, what is required for our solution. Note that, RWMutex does also has a `lock` which is
simply mean write lock. Now with our lock understanding, we have solved most of our problem.


###### Transaction API

Now lets create a simple `lock` and `unlock` API, which internally handles RWMutex locks:


```
func (tx *Tx) lock() {
	if tx.writable {
		tx.db.mu.Lock()
	} else {
		tx.db.mu.RLock()
	}
}

func (tx *Tx) unlock() {
	if tx.writable {
		tx.db.mu.Unlock()
	} else {
		tx.db.mu.RUnlock()
	}
}
```

We are checking the `tx.writable`, if true, we are acquiring the `Lock()` (for write)
else `RLock()` (for read). The same for `unlock`

###### Database API

Create a simple API called `managed` :

```
func (db *DB) managed(writable bool, fn func(tx *Tx) error) (err error) {
	var tx *Tx
	tx, err = db.Begin(writable)
	if err != nil {
		return
	}

	defer func() {
		if writable {
			fmt.Println("Write Unlocking...")
			tx.unlock()
		} else {
			fmt.Println("Read Unlocking...")
			tx.unlock()
		}
	}()

	err = fn(tx)
	return
}
```

it begins the transaction and has a `defer` func which make sure we call our transaction API's
`unlock` function when our function returns. `Begin` function does acquire the lock for us:

```
func (db *DB) Begin(writable bool) (*Tx,error) {
	tx := &Tx {
		db : db,
		writable: writable,
	}
	tx.lock()

	return tx,nil
}
```

(either in read or write mode) based on `writable` and calls transaction's `lock`.

###### Helper API

Now finally, we will provide a helper API for our database calls:

```
func (db * DB) View(fn func (tx *Tx) error) error {
	return db.managed(false, fn)
}

func (db * DB) Update(fn func (tx *Tx) error) error {
	return db.managed(true, fn)
}
```

which in turn calls `managed` with appropriate parameter for `View` & `Update`.

##### Using our simple database:

```
func main() {

	db := Create()

	go db.Update(func(tx *Tx) error {
		tx.Set("mykey", "go")
		tx.Set("mykey2", "is")
		tx.Set("mykey3", "awesome")
		return nil
	})

	go db.View(func(tx *Tx) error {
		fmt.Println(tx.Get("mykey3"))
		return nil
	})

	time.Sleep(200000) //to see the output
}
```

Here we are creating two goroutines which will concurrently/parallel based on your machine core configs.
Try spawning many goroutines and see it in action our simple concurrent safe in memory database.

Remember that, all operation inside either `Update` or `View` is concurrently safe. 

The entire source code is at the file `simple_db.go`.

###### Disclaimer

This database is only for fun, not fit for production use cases or replacing the concurrent sync.Map!
Have a golang day!

###### Help

If you find any grammatical mistake, please raise a PR, would be happy to accept.