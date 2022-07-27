package update

import (
	"database/sql"
	"fmt"
	"os"
	"path"
	"runtime"

	"github.com/lxc/lxd/lxd/db/schema"
)

// CreateSchema is the default schema applied when bootstrapping the database.
const CreateSchema = `
CREATE TABLE schemas (
  id          INTEGER    PRIMARY  KEY    AUTOINCREMENT  NOT  NULL,
  version     INTEGER    NOT      NULL,
  updated_at  DATETIME   NOT      NULL,
  UNIQUE      (version)
);
`

// Template for schema files (can't use backticks since we need to use backticks
// inside the template itself).
const dotGoTemplate = "package %s\n\n" +
	"// DO NOT EDIT BY HAND\n" +
	"//\n" +
	"// This code was generated by the schema.DotGo function. If you need to\n" +
	"// modify the database schema, please add a new schema update to update.go\n" +
	"// and the run 'make update-schema'.\n" +
	"const freshSchema = `\n" +
	"%s`\n"

func Schema() *SchemaUpdate {
	schema := NewFromMap(updates)
	schema.Fresh("")
	return schema
}

func AppendSchema(extensions map[int]schema.Update) {
	currentVersion := len(updates)
	for _, extension := range extensions {
		updates[currentVersion+1] = extension
		currentVersion = len(updates)
	}
}

func SchemaDotGo() error {
	// Apply all the updates that we have on a pristine database and dump
	// the resulting schema.
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return fmt.Errorf("failed to open schema.go for writing: %w", err)
	}

	schema := NewFromMap(updates)

	_, err = schema.Ensure(db)
	if err != nil {
		return err
	}

	dump, err := schema.Dump(db)
	if err != nil {
		return err
	}

	// Passing 1 to runtime.Caller identifies our caller.
	_, filename, _, _ := runtime.Caller(1)

	file, err := os.Create(path.Join(path.Dir(filename), "schema_update.go"))
	if err != nil {
		return fmt.Errorf("failed to open Go file for writing: %w", err)
	}

	pkg := path.Base(path.Dir(filename))
	_, err = file.Write([]byte(fmt.Sprintf(dotGoTemplate, pkg, dump)))
	if err != nil {
		return fmt.Errorf("failed to write to Go file: %w", err)
	}

	return nil
}

var updates = map[int]schema.Update{
	1: updateFromV0,
}

func updateFromV0(tx *sql.Tx) error {
	stmt := fmt.Sprintf(`
%s

CREATE TABLE internal_token_records (
  id           INTEGER         PRIMARY  KEY    AUTOINCREMENT  NOT  NULL,
  joiner_cert  TEXT            NOT      NULL,
  token        TEXT            NOT      NULL,
  UNIQUE       (joiner_cert),
  UNIQUE       (token)
);

CREATE TABLE internal_cluster_members (
  id                   INTEGER   PRIMARY  KEY    AUTOINCREMENT  NOT  NULL,
  name                 TEXT      NOT      NULL,
  address              TEXT      NOT      NULL,
  certificate          TEXT      NOT      NULL,
  schema               INTEGER   NOT      NULL,
  heartbeat            DATETIME  NOT      NULL,
  role                 TEXT      NOT      NULL,
  UNIQUE(name),
  UNIQUE(certificate)
);
`, CreateSchema)

	_, err := tx.Exec(stmt)
	return err
}
