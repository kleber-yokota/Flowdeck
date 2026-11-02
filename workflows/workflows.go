// Package workflows holds the durable workflows: one subpackage per
// workflow (workflows/<name>/ — its own folder, its own file, its own init
// registering on the core DBOS context, core.Ctx()) and the shared step
// functions (workflows/steps/, the leaf of the tree: imported by the
// workflow subpackages, no workflow in it).
//
// New workflow subpackage (workflows/<name>/): add the files, then run
//
//	go generate ./workflows
//
// which rewrites zz_generated.go (the blank imports that pull the
// subpackages into the binary). main.go never changes for a new workflow.
//
//go:generate go run flowdeck/cmd/workflowgen

package workflows
