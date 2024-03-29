// Package merkle provides an implementation of a merkel tree for validation
// support for the blockchain.
package merkle

// Copyright 2017 Cameron Bergoon
// https://github.com/cbergoon/merkletree
// Licensed under the MIT License, see LICENCE file for details.
// This code has be refined for generics.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Hashable represents the behavior concrete data must exhibit to be used in
// the merkle tree.
type Hashable[T any] interface {
	Hash() ([]byte, error)
	Equals(other T) bool
}

// /////////////////////////////////////////////////////////////////

// Tree represents a merkle tree that uses data of some type T that exhibits the
// behavior defined by the Hashable constraint.
type Tree[T Hashable[T]] struct {
	Root         *Node[T]
	Leaves       []*Node[T]
	MerkleRoot   []byte
	hashStrategy func() hash.Hash
}

// WithHashStrategy is used to change the default hash strategy of using sha256
// when constructing a new tree.
func WithHashStrategy[T Hashable[T]](hashStrategy func() hash.Hash) func(t *Tree[T]) {
	return func(t *Tree[T]) {
		t.hashStrategy = hashStrategy
	}
}

// NewTree constructs a new merkle tree that uses data of some type T that
// exhibits the behavior defined by the Hashable interface.
func NewTree[T Hashable[T]](data []T, options ...func(t *Tree[T])) (*Tree[T], error) {
	var defaultHashStrategy = sha256.New
	t := Tree[T]{
		hashStrategy: defaultHashStrategy,
	}

	for _, option := range options {
		option(&t)
	}

	if err := t.Generate(data); err != nil {
		return nil, err
	}

	return &t, nil
}

// Generate constructs the leaves and nodes of the tree from the specified
// data. If the tree has been previously generated, it is re-generated
// from scratch.
func (t *Tree[T]) Generate(values []T) error {
	if len(values) == 0 {
		return errors.New("can't construct tree with no data")
	}

	var leaves []*Node[T]
	for _, value := range values {
		hash, err := value.Hash()
		if err != nil {
			return err
		}

		leaves = append(leaves, &Node[T]{
			Hash:  hash,
			Value: value,
			leaf:  true,
			Tree:  t,
		})
	}

	if len(leaves)%2 == 1 {
		duplicate := &Node[T]{
			Hash:  leaves[len(leaves)-1].Hash,
			Value: leaves[len(leaves)-1].Value,
			leaf:  true,
			dup:   true,
			Tree:  t,
		}
		leaves = append(leaves, duplicate)
	}

	root, err := buildIntermediate(leaves, t)
	if err != nil {
		return err
	}

	t.Root = root
	t.Leaves = leaves
	t.MerkleRoot = root.Hash

	return nil
}

// Rebuild is a helper function that will rebuild the tree
// reusing only the data that it currently holds in the leaves.
func (t *Tree[T]) Rebuild() error {
	var data []T
	for _, node := range t.Leaves {
		data = append(data, node.Value)
	}

	if err := t.Generate(data); err != nil {
		return err
	}

	return nil
}

// Proof returns the set of hashes and the order of concatenating those
// hashes for proving a transaction is in the tree. This is how you can use
// the information returned by this function.
//
// Hash the data in question and know the merkle tree root hash.
// dataHash = "0x8e4c64afaeb4e6210a65eb7a54e51d90d20112a4c085209d3db12f0597f16fd6"
// merkle_root = "0xbc43b5296b8adc75aea5f1d9220bf3bc9dc0dbed9a75d367784b50a7bbbd1211"
//
// Given this proof and proof order from this function for the data in question.
// proof = [
//
//	"0x23d2d2f2a0cbfb260492d42604728cdf8fd63b7d84e4a58094b90dbdd103cd23",
//	"0xdf25fb5ab5d1373ed6e260ead0a5c7b5fc78b0e9ccf9e09407a67bd2faaf3120",
//	"0x9dc3d2d31256f20044646614d0a6326627ccc5f1c42019c552c5929a5b9170f3"]
//
// proof_order = [0, 1, 1]
//
// Process the dataHash against the proof like this.
// bytes = concat(proof[0], dataHash)  -- Order 0 says proof comes first.
//
//	sha1 = sha256.Sum256(bytes)
//
// bytes = concat(sha1, proof[1])      -- Order 1 says proof comes second.
//
//	sha2 = sha256.Sum256(bytes)
//
// bytes = concat(sha2, proof[2])      -- Order 1 says proof comes second.
//
//	root = sha256.Sum256(bytes)
//
// The calculated root should match merkle_root.
func (t *Tree[T]) Proof(data T) ([][]byte, []int64, error) {
	for _, node := range t.Leaves {
		if !node.Value.Equals(data) {
			continue
		}

		var merkleProof [][]byte
		var order []int64
		nodeParent := node.Parent

		for nodeParent != nil {
			if bytes.Equal(nodeParent.Left.Hash, node.Hash) {
				merkleProof = append(merkleProof, nodeParent.Right.Hash)
				order = append(order, 1) // right leaf
			} else {
				merkleProof = append(merkleProof, nodeParent.Left.Hash)
				order = append(order, 0) // left leaf
			}
			node = nodeParent
			nodeParent = nodeParent.Parent
		}

		return merkleProof, order, nil
	}

	return nil, nil, errors.New("unable to find data in tree")
}

// Verify validates the hashes at each level of the tree and
// returns true if the resulting hash at the root of the tree
// matches the resulting root hash; returns false if otherwise.
func (t *Tree[T]) Verify() error {
	calculatedMerkleRoot, err := t.Root.verify()
	if err != nil {
		return err
	}

	if !bytes.Equal(t.MerkleRoot, calculatedMerkleRoot) {
		return errors.New("root hash invalid")
	}

	return nil
}

// VerifyData indicates if a given piece of data is in the tree and the hashes
// are still valid for that data. Returns true if the expected Merkle Root is
// equivalent to the Merkle root calculated on the critical path for that
// data. Returns true if valid and false otherwise.
func (t *Tree[T]) VerifyData(data T) error {
	for _, node := range t.Leaves {
		if !node.Value.Equals(data) {
			continue
		}

		currentParent := node.Parent
		for currentParent != nil {
			rightBytes, err := currentParent.Right.CalculateHash()
			if err != nil {
				return err
			}

			leftBytes, err := currentParent.Left.CalculateHash()
			if err != nil {
				return err
			}

			h := t.hashStrategy()
			if _, err := h.Write(append(leftBytes, rightBytes...)); err != nil {
				return err
			}

			if !bytes.Equal(h.Sum(nil), currentParent.Hash) {
				return errors.New("markle root is not equivalent to the merkle root calculated on the critical path")
			}

			currentParent = currentParent.Parent
		}

		return nil
	}

	return errors.New("markle root is not equivalent to the merkle root calculated on the critical path")
}

// Values returns a slice of unique values stored in the tree.
func (t *Tree[T]) Values() []T {
	var values []T
	for _, tx := range t.Leaves {
		values = append(values, tx.Value)
	}

	l := len(t.Leaves)
	if bytes.Equal(t.Leaves[l-1].Hash, t.Leaves[l-2].Hash) {
		return values[:l-1]
	}

	return values
}

// RootHex converts the merkle root byte hash to a hex encoded string.
func (t *Tree[T]) RootHex() string {
	return hexutil.Encode(t.MerkleRoot)
}

// String returns a string representation of the tree.
// Only leaf nodes are included in the output.
func (t *Tree[T]) String() string {
	s := ""

	for _, l := range t.Leaves {
		s += fmt.Sprint(l)
		s += "\n"
	}

	return s
}

// MarshalText implements the TextMarshaler inteerface and produces a panic
// if anyone tries to marshal the Merkle tree. I don't want this to happen.
// Use the Values function to return a slice that can be marshaled.
func (t *Tree[T]) MarshalText() (text []byte, err error) {
	panic("do not marshal the merkle tree, use Values")
}

// /////////////////////////////////////////////////////////////////

// Node represents a nod, root, or leaf in the tree. It stores pointers to its
// immediate relationships, a hash, the data stored if it's a leaf, and
// other metadata.
type Node[T Hashable[T]] struct {
	Tree   *Tree[T]
	Parent *Node[T]
	Left   *Node[T]
	Right  *Node[T]
	Hash   []byte
	Value  T
	leaf   bool
	dup    bool
}

// verify walks down the tree until hitting a leaf, calculating the
// hash at each level and returning the resulting hash of the Node.
func (n *Node[T]) verify() ([]byte, error) {
	if n.leaf {
		return n.Value.Hash()
	}

	rightBytes, err := n.Right.verify()
	if err != nil {
		return nil, err
	}

	leftBytes, err := n.Left.verify()
	if err != nil {
		return nil, err
	}

	h := n.Tree.hashStrategy()
	if _, err := h.Write(append(leftBytes, rightBytes...)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// CalculateHash is a helper function that calculates the hash of the node.
func (n *Node[T]) CalculateHash() ([]byte, error) {
	if n.leaf {
		return n.Value.Hash()
	}

	h := n.Tree.hashStrategy()
	if _, err := h.Write(append(n.Left.Hash, n.Right.Hash...)); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// String returns a string representation of the node.
func (n *Node[T]) String() string {
	return fmt.Sprintf("%t %t %v %v", n.leaf, n.dup, n.Hash, n.Value)
}

// /////////////////////////////////////////////////////////////////

// buildIntermediate is a helper function that for a given list of leaf nodes
// constructs the intermediate and root levels of the tree. It returns the
// resulting root node of the tree.
func buildIntermediate[T Hashable[T]](nl []*Node[T], t *Tree[T]) (*Node[T], error) {
	var nodes []*Node[T]

	for i := 0; i < len(nl); i += 2 {
		left, right := i, i+1
		if i+1 == len(nl) {
			right = i
		}

		h := t.hashStrategy()
		chash := append(nl[left].Hash, nl[right].Hash...)
		if _, err := h.Write(chash); err != nil {
			return nil, err
		}

		n := Node[T]{
			Left:  nl[left],
			Right: nl[right],
			Hash:  h.Sum(nil),
			Tree:  t,
		}

		nodes = append(nodes, &n)
		nl[left].Parent = &n
		nl[right].Parent = &n

		if len(nl) == 2 {
			return &n, nil
		}
	}

	return buildIntermediate(nodes, t)
}
