package notdiamond

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"
)

// contains is a helper to check if a slice of strings contains a given item.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// TestOpenNewDatabase verifies that a new database is created and contains the default "keystore" table.
func TestOpenNewDatabase(t *testing.T) {
	dbName := "testdb_open"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	// Check that the keystore table exists.
	cols, err := db.Columns("keystore")
	if err != nil {
		t.Errorf("Columns() failed: %v", err)
	}
	if len(cols) == 0 {
		t.Error("keystore table should have columns")
	}
	if err := db.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

// TestMakeTables creates both a non–timeseries table and a timeseries table and verifies their column names.
func TestMakeTables(t *testing.T) {
	db, err := Open("testdb_make", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Create a non–timeseries table.
	nonTSTable := "non_ts"
	columns := map[string]string{
		"value": "TEXT",
	}
	if err := db.MakeTables(false, nonTSTable, columns); err != nil {
		t.Fatalf("MakeTables(non-timeseries) failed: %v", err)
	}
	cols, err := db.Columns(nonTSTable)
	if err != nil {
		t.Fatalf("Columns(non_ts) failed: %v", err)
	}
	// For non-timeseries tables, expect a primary key column "key" plus our custom columns.
	if !contains(cols, "key") {
		t.Errorf("Expected column 'key' in table %s", nonTSTable)
	}
	if !contains(cols, "value") {
		t.Errorf("Expected column 'value' in table %s", nonTSTable)
	}

	// Create a timeseries table.
	tsTable := "ts_table"
	columnsTS := map[string]string{
		"metric": "REAL",
	}
	if err := db.MakeTables(true, tsTable, columnsTS); err != nil {
		t.Fatalf("MakeTables(timeseries) failed: %v", err)
	}
	cols, err = db.Columns(tsTable)
	if err != nil {
		t.Fatalf("Columns(ts_table) failed: %v", err)
	}
	// For timeseries tables, expect "id", "timestamp" plus custom columns.
	if !contains(cols, "id") {
		t.Errorf("Expected column 'id' in timeseries table")
	}
	if !contains(cols, "timestamp") {
		t.Errorf("Expected column 'timestamp' in timeseries table")
	}
	if !contains(cols, "metric") {
		t.Errorf("Expected column 'metric' in timeseries table")
	}
}

// TestSetGetDelete exercises the Set, Get, and Delete operations.
func TestSetGetDelete(t *testing.T) {
	db, err := Open("testdb_setget", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	tableName := "keystore"
	key := "testKey"
	value := "testValue"

	// Test Set.
	if err := db.Set(tableName, key, value); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Test Get.
	var got string
	if err := db.Get(tableName, key, &got); err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if got != value {
		t.Errorf("Get() returned %v; want %v", got, value)
	}

	// Test Delete.
	if err := db.Delete(tableName, key); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}
	// After deletion, Get should return an error.
	if err := db.Get(tableName, key, &got); err == nil {
		t.Errorf("Expected error after Delete, got nil")
	}
}

// TestDump verifies that Dump returns an SQL dump containing known table names.
func TestDump(t *testing.T) {
	db, err := Open("testdb_dump", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	// Insert a value into keystore.
	key := "dumpKey"
	value := "dumpValue"
	if err := db.Set("keystore", key, value); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	dump, err := db.Dump()
	if err != nil {
		t.Fatalf("Dump() failed: %v", err)
	}
	if !strings.Contains(dump, "keystore") {
		t.Errorf("Dump() does not contain 'keystore' table definition")
	}
}

// TestExists verifies the Exists function for both an existing and a non-existing database.
func TestExists(t *testing.T) {
	dbName := "existsdb"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	err = db.Close()
	if err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Exists should succeed.
	if err := Exists(dbName); err != nil {
		t.Errorf("Exists() returned error for existing DB: %v", err)
	}

	// For a non-existent database, Exists should return an error.
	nonExisting := "nonexistentdb"
	if err := Exists(nonExisting); err == nil {
		t.Errorf("Exists() expected error for non-existing DB")
	}
}

// TestDrop ensures that Drop closes the DB, releases the lock, and removes the file.
func TestDrop(t *testing.T) {
	dbName := "dropdb"
	db, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	dbFile := db.Schema

	if err := db.Drop(); err != nil {
		t.Fatalf("Drop() failed: %v", err)
	}
	// Verify that the file no longer exists.
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("Database file still exists after Drop()")
	}
}

// TestQueryExec tests Exec and Query by creating a temporary table, inserting a row, and querying it.
func TestQueryExec(t *testing.T) {
	db, err := Open("testdb_queryexec", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	createStmt := `CREATE TABLE IF NOT EXISTS temp_table (id INTEGER PRIMARY KEY, name TEXT);`
	if err := db.Exec(createStmt); err != nil {
		t.Fatalf("Exec(create table) failed: %v", err)
	}

	insertStmt := `INSERT INTO temp_table (name) VALUES (?);`
	if err := db.Exec(insertStmt, "Alice"); err != nil {
		t.Fatalf("Exec(insert) failed: %v", err)
	}

	rows, err := db.Query("SELECT name FROM temp_table WHERE name = ?", "Alice")
	if err != nil {
		t.Fatalf("Query() failed: %v", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	var name string
	if rows.Next() {
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Row Scan failed: %v", err)
		}
		if name != "Alice" {
			t.Errorf("Expected 'Alice', got %v", name)
		}
	} else {
		t.Errorf("No rows returned from Query()")
	}
}

// TestInvalidQuery ensures that Exec returns an error for an invalid SQL statement.
func TestInvalidQuery(t *testing.T) {
	db, err := Open("testdb_invalidquery", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	err = db.Exec("INVALID SQL STATEMENT")
	if err == nil {
		t.Errorf("Expected error for invalid SQL statement, got nil")
	}
}

// TestConcurrentOpen verifies that concurrent attempts to open the same DB wait until the lock is released.
func TestConcurrentOpen(t *testing.T) {
	dbName := "concurrentdb"
	// Open the first instance.
	db1, err := Open(dbName, false)
	if err != nil {
		t.Fatalf("Open() db1 failed: %v", err)
	}
	// Do not close db1 immediately so that it holds the lock.
	var db2 *Database
	var err2 error
	done := make(chan struct{})
	go func() {
		// This call should block until db1 is closed.
		db2, err2 = Open(dbName, false)
		close(done)
	}()

	// Wait a moment to ensure the goroutine is blocked.
	time.Sleep(50 * time.Millisecond)
	if err := db1.Close(); err != nil {
		t.Fatalf("Close() db1 failed: %v", err)
	}

	select {
	case <-done:
		if err2 != nil {
			t.Errorf("Open() db2 failed: %v", err2)
		}
		if db2 == nil {
			t.Errorf("db2 is nil")
		} else {
			err := db2.Close()
			if err != nil {
				return
			}
		}
	case <-time.After(2 * time.Second):
		t.Errorf("Timeout waiting for concurrent Open() to complete")
	}
}

// TestCloseIdempotent verifies that calling Close multiple times does not produce an error.
func TestCloseIdempotent(t *testing.T) {
	db, err := Open("testdb_close", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("First Close() failed: %v", err)
	}
	// Second Close() should be a no-op.
	if err := db.Close(); err != nil {
		t.Errorf("Second Close() returned error: %v", err)
	}
}

// TestDebug calls Debug to verify no panic occurs.
func TestDebug(t *testing.T) {
	db, err := Open("testdb_debug", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)
	db.Debug(true)
	db.Debug(false)
}

// TestGetNonexistentKey verifies that Get returns an error when a key does not exist.
func TestGetNonexistentKey(t *testing.T) {
	db, err := Open("testdb_nonexistent", false)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer func(db *Database) {
		err := db.Close()
		if err != nil {

		}
	}(db)

	var result string
	err = db.Get("keystore", "nonexistent", &result)
	if err == nil {
		t.Errorf("Expected error for non-existent key, got nil")
	}
}
