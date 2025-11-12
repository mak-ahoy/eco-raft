package raftkv

import "labrpc"
import "crypto/rand"
import "math/big"
import "time"


type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	clientID int64 // Unique identifier for the client
	requestID int64 // Incrementing request ID
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	ck.clientID = nrand() // Assign a unique client ID
	ck.requestID = 0     // Start with requestID = 0
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//


// Get retrieves the value associated with the key, retries on failure
func (ck *Clerk) Get(key string) string {
	args := GetArgs{
		Key:       key,
		ClientID:  ck.clientID,
		RequestID: ck.incrementRequestID(),
	}

	
	for {
		// Try all servers until success
		for _, srv := range ck.servers {
			var reply GetReply
			ok := srv.Call("RaftKV.Get", &args, &reply)
			
			if reply.WrongLeader{
				continue
			}
			if ok && reply.Err == "" {
				return reply.Value // Return the value if no error
			} else{
				return ""
			}

		}
		// If all servers fail, wait before retrying
		time.Sleep(time.Millisecond * 100)
	}
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("RaftKV.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
// func (ck *Clerk) PutAppend(key string, value string, op string) {
// 	// You will have to modify this function.
// }


// PutAppend shared method for both Put and Append
func (ck *Clerk) PutAppend(key string, value string, op string) {
	args := PutAppendArgs{
		Key:       key,
		Value:     value,
		Op:        op,
		ClientID:  ck.clientID,
		RequestID: ck.incrementRequestID(),
	}

	
	for {
		// Try all servers until success
		for _, srv := range ck.servers {
			var reply PutAppendReply

			ok := srv.Call("RaftKV.PutAppend", &args, &reply)
			if reply.WrongLeader{
				continue
			}
			if ok && reply.Err == "" {
				return // Success, exit the loop
			}
		}
		// If all servers fail, wait before retrying
		time.Sleep(time.Millisecond * 100)
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}


// incrementRequestID safely increments and returns the request ID
func (ck *Clerk) incrementRequestID() int64 {
	ck.requestID++
	return ck.requestID
}

// The following structs are used for RPC argument and response definitions:



