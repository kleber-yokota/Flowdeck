package graph

import (
	"strings"
)

// sequence/loop block's children extend every open path, an if forks). Empty
// sequences and duplicates are dropped.
func readPaths(b *Block) [][]string {
	return dedupePaths(readBlock(b, [][]string{{}}))
}

// readBlock reads the block's children into the open paths (forking at the
// nested ifs).
func readBlock(b *Block, open [][]string) [][]string {
	for _, c := range b.Children {
		open = readChild(c, open)
	}
	return open
}

// readChild reads one child into the open paths: a node appends its name, an
// if forks (taken vs not-taken), a sequence/loop block's children extend
// every open path.
func readChild(c Child, open [][]string) [][]string {
	if c.Node != nil {
		return addStepToAll(open, c.Node.Name)
	}
	b := c.Block
	if b.Type == BlockIf {
		taken := readBlock(b, [][]string{{}})
		notTaken := blockSequences(b.Else)
		return forkPaths(open, taken, notTaken)
	}
	return readBlock(b, open)
}

// blockSequences returns the sequences of step names through the else (the
// not-taken branch of an if): nil is the empty sequence, an else-if forks, an
// else body extends.
func blockSequences(b *Block) [][]string {
	if b == nil {
		return [][]string{{}}
	}
	if b.Type == BlockIf {
		return forkPaths([][]string{{}}, readBlock(b, [][]string{{}}), blockSequences(b.Else))
	}
	return readBlock(b, [][]string{{}})
}

// stepsOf returns the leaf node names of the block tree, in build order.
func stepsOf(b *Block) []string {
	var steps []string
	var walk func(bl *Block)
	walk = func(bl *Block) {
		for _, c := range bl.Children {
			if c.Node != nil {
				steps = append(steps, c.Node.Name)
			} else {
				walk(c.Block)
			}
		}
		if bl.Else != nil {
			walk(bl.Else)
		}
	}
	walk(b)
	return steps
}

// forkPaths splits every open path into taken (p + each taken sequence) and
// not-taken (p + each not-taken sequence). A branch with no RunAsStep is
// the empty sequence (the path keeps going unchanged).
func forkPaths(base, taken, notTaken [][]string) [][]string {
	out := make([][]string, 0, len(base)*2)
	for _, p := range base {
		for _, t := range taken {
			out = append(out, joinSteps(p, t))
		}
		for _, n := range notTaken {
			out = append(out, joinSteps(p, n))
		}
	}
	return out
}

func joinSteps(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	return append(out, extra...)
}

func addStepToAll(paths [][]string, steps ...string) [][]string {
	for i, p := range paths {
		paths[i] = append(append([]string(nil), p...), steps...)
	}
	return paths
}

// dedupePaths drops duplicate sequences and empty ones (a workflow that
// ran no steps is not a path).
func dedupePaths(paths [][]string) [][]string {
	seen := make(map[string]struct{}, len(paths))
	out := make([][]string, 0, len(paths))
	for _, p := range paths {
		if len(p) == 0 {
			continue
		}
		key := strings.Join(p, "\x00")
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}
