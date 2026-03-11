package memtable

import (
	"fmt"
	"math/rand/v2"
	"sync"
)

const maxLevel = 32
const delKey = "___TOMBSTONE___"

type Node struct {
	Key     string
	Value   []byte
	forward []*Node
}
type KVPair struct {
	Key   string
	Value []byte
}

// Makes node with a given height
func NewNode(key string, value []byte, level int) *Node {
	return &Node{
		Key:     key,
		Value:   value,
		forward: make([]*Node, level),
	}
}

type SkipList struct {
	head  *Node
	level int
	rwmu  sync.RWMutex
}

func NewSkipList() *SkipList {
	// rand.Seed()
	// this is not required now as go automatically start with a complete random starting position
	return &SkipList{
		head:  NewNode("", nil, maxLevel),
		level: 1,
	}
}

// level generator random function
func (s *SkipList) randomLevel() int {
	level := 1
	// rand.Float32() returns a decimal between 0.0 and 1.0.
	// Checking if it is < 0.5 gives us a 50% coin flip.
	for rand.Float32() < 0.5 && level < maxLevel {
		level++
	}
	return level
}

// get method
func (s *SkipList) Get(key string) ([]byte, bool) {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()
	node := s.head
	curLvl := s.level - 1
	for curLvl >= 0 {
		for {
			// multiple forward step at the current level till we can go further
			nextUp := node.forward[curLvl]
			if nextUp == nil {
				break
			}
			if nextUp.Key < key {
				node = nextUp
			} else {
				break
			}
		}
		curLvl--
	}
	if node.forward[0] == nil {
		return nil, false
	}
	if string(node.forward[0].Key) == key {
		if string(node.forward[0].Value) == delKey {
			return nil, false
		}
		return node.forward[0].Value, true
	}
	return nil, false

}

func (s *SkipList) Put(key string, value []byte) {
	// write lock
	// as memory manipulation is there so this one should be a exculive lock
	s.rwmu.Lock()
	defer s.rwmu.Unlock()
	curNode := s.head
	curLvl := s.level - 1
	previous := make([]*Node, maxLevel)
	// intializing previous to s.head
	for i := 0; i < maxLevel; i++ {
		previous[i] = s.head
	}
	for curLvl >= 0 {
		for {
			temp := curNode.forward[curLvl]
			if temp == nil {
				previous[curLvl] = curNode
				break
			}
			if temp.Key < key {
				curNode = temp
			} else {
				previous[curLvl] = curNode
				break
			}
		}
		curLvl--
	}
	dest := curNode.forward[0]
	// exist and we just have to update it
	if dest != nil && dest.Key == key {

		dest.Value = value
		return
	}
	lvl := s.randomLevel()
	s.level = max(lvl, s.level)
	neoNode := NewNode(key, value, lvl)

	for i := 0; i < lvl; i++ {
		neoNode.forward[i] = previous[i].forward[i]
		previous[i].forward[i] = neoNode
	}

}

func (s *SkipList) PrintAll() {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()

	node := s.head.forward[0]
	fmt.Println("--- Database Contents ---")
	for node != nil {
		fmt.Printf("Key: %s, Value: %s\n", node.Key, string(node.Value))
		node = node.forward[0]
	}
	fmt.Println("-------------------------")
}

func (s *SkipList) GetAll() []KVPair {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()

	var pairs []KVPair
	node := s.head.forward[0] // Start at the first real node on Level 0

	for node != nil {
		pairs = append(pairs, KVPair{
			Key:   node.Key,
			Value: node.Value,
		})
		node = node.forward[0]
	}

	return pairs
}
