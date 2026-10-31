package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"flowdeck/core/graph"
	webgraph "flowdeck/web/graph"
)

func main() {
	fsys := os.DirFS("workflows")
	g, err := graph.New(fsys)
	if err != nil {
		panic(err)
	}
	names := make([]string, 0, len(g.ByName))
	for name := range g.ByName {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		wg := g.ByName[name]
		fmt.Printf("=== %s ===\n", name)
		printBlock(wg.Root, 0)
		fmt.Print("paths: ")
		for i, p := range wg.Paths {
			if i > 0 {
				fmt.Print(" | ")
			}
			fmt.Print(strings.Join(p, " -> "))
		}
		fmt.Println()
		fmt.Println("svg:")
		fmt.Print(webgraph.BlockToSVG(wg))
		fmt.Println()
	}
}

func printBlock(b *graph.Block, indent int) {
	if b == nil {
		return
	}
	pad := ""
	for i := 0; i < indent; i++ {
		pad += "  "
	}
	fmt.Printf("%s%s [%s L%s]\n", pad, b.ID, b.Type, strconv.Itoa(b.Level))
	for _, c := range b.Children {
		if c.Node != nil {
			fmt.Printf("%s  node %s (%s)\n", pad, c.Node.Name, c.Node.Kind)
		} else if c.Block != nil {
			printBlock(c.Block, indent+1)
		}
	}
	if b.Else != nil {
		fmt.Printf("%s  ELSE:\n", pad)
		printBlock(b.Else, indent+1)
	}
}
