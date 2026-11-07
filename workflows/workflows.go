// Package workflows holds the durable workflows: one subpackage per
// workflow (workflows/<name>/ — its own folder, its own file, its own
// Register(c) that registers on the core DBOS context) and the shared
// step functions (workflows/steps/, the leaf of the tree: imported by the
// workflow subpackages, no workflow in it).
//
// New workflow subpackage (workflows/<name>/): add the files (including
// func Register(c dbos.Context)), then run
//
//	go generate ./workflows
//
// which rewrites zz_generated.go (the generated RegisterAll call list —
// the registration, one call per subpackage). main.go never changes for a
// new workflow.
//
//go:generate go run flowdeck/cmd/workflowgen

package workflows
