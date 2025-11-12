package raftkv

import (
	"encoding/gob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"time"
)

const Debug = 1  // Set to 1 for debugging, 0 to turn off

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Operation type: Get, Put, Append
	OpType string
	// Key and value for Put/Append
	Key   string
	Value string
	// Client and request ID for idempotency
	ClientID  int64
	RequestID int64
}

type RaftKV struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	clientRequests map[int64]map[int64]bool
	waitChans      map[int]chan ApplyResult

	data map[string]string // Store the key-value data
}

type ApplyResult struct {
	Err   Err
	Value string
}

func (kv *RaftKV) Get(args *GetArgs, reply *GetReply) {
	//("RaftKV[%d] received Get request: Key=%s, ClientID=%d, RequestID=%d", kv.me, args.Key, args.ClientID, args.RequestID)

	op := Op{
		OpType:    "Get",
		Key:       args.Key,
		ClientID:  args.ClientID,
		RequestID: args.RequestID,
	}

	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		//("RaftKV[%d] is not the leader. Responding with WrongLeader.", kv.me)
		return
	}

	ch := make(chan ApplyResult, 1)
	kv.waitChans[index] = ch

	select {
	case res := <-ch:
		reply.Err = res.Err
		reply.Value = res.Value
		//("RaftKV[%d] completed Get operation: Key=%s, Value=%s, Err=%s", kv.me, args.Key, res.Value, res.Err)
	case <-time.After(500 * time.Millisecond):
		reply.WrongLeader = true
		//("RaftKV[%d] Get request timed out. Responding with WrongLeader.", kv.me)
	}
}

func (kv *RaftKV) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	//("RaftKV[%d] received Put/Append request: Op=%s, Key=%s, Value=%s, ClientID=%d, RequestID=%d", kv.me, args.Op, args.Key, args.Value, args.ClientID, args.RequestID)

	op := Op{
		OpType:    args.Op,
		Key:       args.Key,
		Value:     args.Value,
		ClientID:  args.ClientID,
		RequestID: args.RequestID,
	}

	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.WrongLeader = true
		//("RaftKV[%d] is not the leader. Responding with WrongLeader.", kv.me)
		return
	}

	ch := make(chan ApplyResult, 1)
	kv.waitChans[index] = ch

	select {
	case res := <-ch:
		reply.Err = res.Err
		//("RaftKV[%d] completed Put/Append operation: Op=%s, Key=%s, Err=%s", kv.me, args.Op, args.Key, res.Err)
	case <-time.After(500 * time.Millisecond):
		reply.WrongLeader = true
		//("RaftKV[%d] Put/Append request timed out. Responding with WrongLeader.", kv.me)
	}
}

func (kv *RaftKV) applyEntries(msg raft.ApplyMsg) {
	op := msg.Command.(Op)
	//("RaftKV[%d] applying entry: Index=%d, OpType=%s, Key=%s, Value=%s", kv.me, msg.Index, op.OpType, op.Key, op.Value)

	if _, ok := kv.clientRequests[op.ClientID][op.RequestID]; !ok {
		var value string
		var err Err
		switch op.OpType {
		case "Put":
			kv.data[op.Key] = op.Value
			err = ""
		case "Append":
			_, ok := kv.data[op.Key]
			if ok {
				kv.data[op.Key] += op.Value
			} else {
				kv.data[op.Key] = op.Value
			}
			err = ""
		case "Get":
			value, _ = kv.data[op.Key]
			err = ""
		}

		if _, ok := kv.clientRequests[op.ClientID]; !ok {
			kv.clientRequests[op.ClientID] = make(map[int64]bool)
		}
		kv.clientRequests[op.ClientID][op.RequestID] = true

		if ch, found := kv.waitChans[msg.Index]; found {
			delete(kv.waitChans, msg.Index)
			ch <- ApplyResult{Err: err, Value: value}
			//("RaftKV[%d] applied entry: Index=%d, Key=%s, Value=%s, Err=%s", kv.me, msg.Index, op.Key, value, err)
		}
	} else {
	
		if ch, found := kv.waitChans[msg.Index]; found {
			delete(kv.waitChans, msg.Index)
			ch <- ApplyResult{Err: "", Value: kv.data[op.Key]}
			//("RaftKV[%d] already applied operation. Returning previous value for Key=%s", kv.me, op.Key)
		}
	}
}

func (kv *RaftKV) Kill() {
	//("RaftKV[%d] is being killed.", kv.me)
	kv.rf.Kill()
}

func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *RaftKV {
	gob.Register(Op{})

	kv := new(RaftKV)
	kv.me = me
	kv.maxraftstate = maxraftstate
	kv.data = make(map[string]string) // Initialize the KV store
	kv.clientRequests = make(map[int64]map[int64]bool)
	kv.waitChans = make(map[int]chan ApplyResult)

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	go func() {
		for msg := range kv.applyCh {
			kv.applyEntries(msg)
		}
	}()

	//("RaftKV[%d] server started.", kv.me)
	return kv
}
