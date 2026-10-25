package core

// The SQLite driver is a hard dependency of this package (not of the test
// files): core.init creates the DBOS context from the environment, and the
// driver's init must run before it — a blank import from a test file is a
// sibling package, initialized in no guaranteed order relative to this
// one. The Postgres runtime path never touches the driver; the cost of the
// guarantee is the modernc.org/sqlite link in the production binary.
import _ "github.com/dbos-inc/dbos-transact-golang/dbos/driver/sqlite"
