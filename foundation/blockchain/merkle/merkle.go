// Copyright 2017 Cameron Bergoon
// https://github.com/cbergoon/merkletree
// Licensed under the MIT License, see LICENCE file for details.
// This code has be refined for generics.

// Package merkle provides an implementation of a merkel tree for validation
// support for the blockchain.

package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
)

// Hashable represents the behavior concrete data must exhibit to be used in
// the merkle tree.
type Hashable[T any] interface {
	Hash() ([]byte, error)
	Equals(other T) (bool, error)
}

// /////////////////////////////////////////////////////////////////

// Tree represents a merkle tree that uses data of some type T that exhibits the
// behavior defined by the Hashable constraint.
type Tree[T Hashable[T]] struct {
	Root         *Node[T]
	MerkleRoot   []byte
	Leaves       []*Node[T]
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
	
	if err := t.GenerateTree(data); err != nil {
		return nil, err
	}
	
	return &t, nil
}

// GenerateTree constructs the leaves and nodes of the tree from the specified
// data. If the tree has been previously generated, it is re-generated
// from scratch.
func (t *Tree[T]) GenerateTree(data []T) error {
	if len(data) == 0 {
		return errors.New("can't construct tree with no data")
	}
	
	var leaves []*Node[T]
	for _, dt := range data {
		hash, err := dt.Hash()
		if err != nil {
			return err
		}
		
		leaves = append(leaves, &Node[T]{
			Hash: hash,
			Data: dt,
			leaf: true,
			Tree: t,
		})
	}
	
	if len(leaves)%2 == 1 {
		duplicate := &Node[T]{
			Hash: leaves[len(leaves)-1].Hash,
			Data: leaves[len(leaves)-1].Data,
			leaf: true,
			dup:  true,
			Tree: t,
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

// RebuildTree is a helper function that will rebuild the tree
// reusing only the data that it currently holds in the leaves.
func (t *Tree[T]) RebuildTree() error {
	var data []T
	for _, node := range t.Leaves {
		data = append(data, node.Data)
	}
	
	if err := t.GenerateTree(data); err != nil {
		return err
	}
	
	return nil
}

// MerklePath gets the tree path and indexes (left leaf or right leaf)
// // for the specified data.
func (t *Tree[T]) MerklePath(data T) ([][]byte, []int64, error) {
	for _, node := range t.Leaves {
		ok, err := node.Data.Equals(data)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		
		nodeParent := node.Parent
		var merklePath [][]byte
		var index []int64
		for nodeParent != nil {
			if bytes.Equal(nodeParent.Left.Hash, node.Hash) {
				merklePath = append(merklePath, nodeParent.Right.Hash)
				index = append(index, 1) // right leaf
			} else {
				merklePath = append(merklePath, nodeParent.Left.Hash)
				index = append(index, 0) // left leaf
			}
			node = nodeParent
			nodeParent = nodeParent.Parent
		}
		
		return merklePath, index, nil
	}
	
	return nil, nil, nil
}

// VerifyTree validates the hashes at each level of the tree and
// returns true if the resulting hash at the root of the tree
// matches the resulting root hash; returns false if otherwise.
func (t *Tree[T]) VerifyTree() error {
	calculatedMerkleRoot, err := t.Root.verifyNode()
	if err != nil {
		return err
	}
	
	if bytes.Equal(t.MerkleRoot, calculatedMerkleRoot) {
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
		ok, err := node.Data.Equals(data)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		
		currentParent := node.Parent
		for currentParent != nil {
			rightBytes, err := currentParent.Right.CalculateNodeHash()
			if err != nil {
				return err
			}
			
			leftBytes, err := currentParent.Left.CalculateNodeHash()
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

// MerkleRootHex provides the hexidecimal encoding of the merkle root.
func (t *Tree[T]) MerkleRootHex() string {
	return hex.EncodeToString(t.MerkleRoot)
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
	Data   T
	leaf   bool
	dup    bool
}

// verifyNode walks down the tree until hitting a leaf, calculating the
// hash at each level and returning the resulting hash of the Node.
func (n *Node[T]) verifyNode() ([]byte, error) {
	if n.leaf {
		return n.Data.Hash()
	}
	
	rightBytes, err := n.Right.verifyNode()
	if err != nil {
		return nil, err
	}
	
	leftBytes, err := n.Left.verifyNode()
	if err != nil {
		return nil, err
	}
	
	h := n.Tree.hashStrategy()
	if _, err := h.Write(append(leftBytes, rightBytes...)); err != nil {
		return nil, err
	}
	
	return h.Sum(nil), nil
}

// CalculateNodeHash is a helper function that calculates the hash of the node.
func (n *Node[T]) CalculateNodeHash() ([]byte, error) {
	if n.leaf {
		return n.Data.Hash()
	}
	
	h := n.Tree.hashStrategy()
	if _, err := h.Write(append(n.Left.Hash, n.Right.Hash...)); err != nil {
		return nil, err
	}
	
	return h.Sum(nil), nil
}

// String returns a string representation of the node.
func (n *Node[T]) String() string {
	return fmt.Sprintf("%t %t %v %v", n.leaf, n.dup, n.Hash, n.Data)
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
