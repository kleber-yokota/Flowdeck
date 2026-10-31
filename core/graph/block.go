package graph

// BlockType is the type of a block: the container in the possible-paths tree.
type BlockType string

const (
	// BlockSequence is an ordered sequence of children (the root, an else body).
	BlockSequence BlockType = "sequence"
	// BlockIf forks: the if-body (Children) vs the not-taken one (Else).
	BlockIf BlockType = "if"
	// BlockLoop repeats its children (a for, a for range).
	BlockLoop BlockType = "loop"
	// BlockJoin groups a run of async nodes and the channel wait (a select /
	// a recv) right after them: the asyncs and the wait that joins them, one
	// level deeper.
	BlockJoin BlockType = "join"
)

// NodeKind is the type of a leaf node: one construct and its name.
type NodeKind string

const (
	// NodeStep is a RunAsStep call.
	NodeStep NodeKind = "step"
	// NodeAsync is a dbos.Go call (an async step, → chan).
	NodeAsync NodeKind = "async"
	// NodeChild is a RunWorkflow call (a child workflow).
	NodeChild NodeKind = "child"
	// NodeWait is a Sleep / Recv / Select call (a wait).
	NodeWait NodeKind = "wait"
)

// Node is a leaf of the block tree: one construct and its name.
type Node struct {
	Name string
	Kind NodeKind
}

// Child is one ordered item in a block: a leaf node (Node) or a nested block
// (Block). Exactly one is set.
type Child struct {
	Node  *Node
	Block *Block
}

// Block is a container in the possible-paths tree: an ordered sequence of
// children (steps and nested blocks). An if forks: Children is the if-body,
// Else is the else (an else-if, an else-body sequence, or nil = the path
// skips). A loop repeats its children.
type Block struct {
	ID       string
	Level    int
	Type     BlockType
	Children []Child
	Else     *Block
}

// WorkflowGraph is the read model of one workflow's possible paths: the
// stored block tree (Root), the read-back paths (Paths), and the steps (the
// leaf node names, Steps).
type WorkflowGraph struct {
	Root  *Block
	Steps []string
	Paths [][]string
}
