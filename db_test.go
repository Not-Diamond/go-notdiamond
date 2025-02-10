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
	db, err := openDB(dbName, false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	// Check that the keystore table exists.
	cols, err := db.getColumns("keystore")
	if err != nil {
		t.Errorf("getColumns() failed: %v", err)
	}
	if len(cols) == 0 {
		t.Error("keystore table should have columns")
	}
	if err := db.closeConnection(); err != nil {
		t.Errorf("closeConnection() failed: %v", err)
	}
}

// TestMakeTables creates both a non–timeseries table and a timeseries table and verifies their column names.
func TestMakeTables(t *testing.T) {
	db, err := openDB("testdb_make", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	// Create a non–timeseries table.
	nonTSTable := "non_ts"
	columns := map[string]string{
		"value": "TEXT",
	}
	if err := db.makeTables(false, nonTSTable, columns); err != nil {
		t.Fatalf("makeTables(non-timeseries) failed: %v", err)
	}
	cols, err := db.getColumns(nonTSTable)
	if err != nil {
		t.Fatalf("getColumns(non_ts) failed: %v", err)
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
	if err := db.makeTables(true, tsTable, columnsTS); err != nil {
		t.Fatalf("makeTables(timeseries) failed: %v", err)
	}
	cols, err = db.getColumns(tsTable)
	if err != nil {
		t.Fatalf("getColumns(ts_table) failed: %v", err)
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

// TestSetGetDelete exercises the setJSON, getJSON, and deleteItem operations.
func TestSetGetDelete(t *testing.T) {
	db, err := openDB("testdb_setget", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	tableName := "keystore"
	key := "testKey"
	value := "testValue"

	// Test setJSON.
	if err := db.setJSON(tableName, key, value); err != nil {
		t.Fatalf("setJSON() failed: %v", err)
	}

	// Test getJSON.
	var got string
	if err := db.getJSON(tableName, key, &got); err != nil {
		t.Fatalf("getJSON() failed: %v", err)
	}
	if got != value {
		t.Errorf("getJSON() returned %v; want %v", got, value)
	}

	// Test deleteItem.
	if err := db.deleteItem(tableName, key); err != nil {
		t.Fatalf("deleteItem() failed: %v", err)
	}
	// After deletion, getJSON should return an error.
	if err := db.getJSON(tableName, key, &got); err == nil {
		t.Errorf("Expected error after deleteItem, got nil")
	}
}

// TestDump verifies that dumpDB returns an SQL dump containing known table names.
func TestDump(t *testing.T) {
	db, err := openDB("testdb_dump", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	// Insert a value into keystore.
	key := "dumpKey"
	value := "dumpValue"
	if err := db.setJSON("keystore", key, value); err != nil {
		t.Fatalf("setJSON() failed: %v", err)
	}

	dump, err := db.dumpDB()
	if err != nil {
		t.Fatalf("dumpDB() failed: %v", err)
	}
	if !strings.Contains(dump, "keystore") {
		t.Errorf("dumpDB() does not contain 'keystore' table definition")
	}
}

// TestExists verifies the entryExists function for both an existing and a non-existing database.
func TestExists(t *testing.T) {
	dbName := "existsdb"
	db, err := openDB(dbName, false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	err = db.closeConnection()
	if err != nil {
		t.Errorf("closeConnection() failed: %v", err)
	}

	// entryExists should succeed.
	if err := entryExists(dbName); err != nil {
		t.Errorf("entryExists() returned error for existing DB: %v", err)
	}

	// For a non-existent database, entryExists should return an error.
	nonExisting := "nonexistentdb"
	if err := entryExists(nonExisting); err == nil {
		t.Errorf("entryExists() expected error for non-existing DB")
	}
}

// TestDrop ensures that dropDB closes the DB, releases the lock, and removes the file.
func TestDrop(t *testing.T) {
	dbName := "dropdb"
	db, err := openDB(dbName, false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	dbFile := db.Schema

	if err := db.dropDB(); err != nil {
		t.Fatalf("dropDB() failed: %v", err)
	}
	// Verify that the file no longer exists.
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("database file still exists after dropDB()")
	}
}

// TestQueryExec tests execQuery and executeQuery by creating a temporary table, inserting a row, and querying it.
func TestQueryExec(t *testing.T) {
	db, err := openDB("testdb_queryexec", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	createStmt := `CREATE TABLE IF NOT EXISTS temp_table (id INTEGER PRIMARY KEY, name TEXT);`
	if err := db.execQuery(createStmt); err != nil {
		t.Fatalf("execQuery(create table) failed: %v", err)
	}

	insertStmt := `INSERT INTO temp_table (name) VALUES (?);`
	if err := db.execQuery(insertStmt, "Alice"); err != nil {
		t.Fatalf("execQuery(insert) failed: %v", err)
	}

	rows, err := db.executeQuery("SELECT name FROM temp_table WHERE name = ?", "Alice")
	if err != nil {
		t.Fatalf("executeQuery() failed: %v", err)
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
		t.Errorf("No rows returned from executeQuery()")
	}
}

// TestInvalidQuery ensures that execQuery returns an error for an invalid SQL statement.
func TestInvalidQuery(t *testing.T) {
	db, err := openDB("testdb_invalidquery", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	err = db.execQuery("INVALID SQL STATEMENT")
	if err == nil {
		t.Errorf("Expected error for invalid SQL statement, got nil")
	}
}

// TestConcurrentOpen verifies that concurrent attempts to open the same DB wait until the lock is released.
func TestConcurrentOpen(t *testing.T) {
	dbName := "concurrentdb"
	// openDB the first instance.
	db1, err := openDB(dbName, false)
	if err != nil {
		t.Fatalf("openDB() db1 failed: %v", err)
	}
	// Do not close db1 immediately so that it holds the lock.
	var db2 *database
	var err2 error
	done := make(chan struct{})
	go func() {
		// This call should block until db1 is closed.
		db2, err2 = openDB(dbName, false)
		close(done)
	}()

	// Wait a moment to ensure the goroutine is blocked.
	time.Sleep(50 * time.Millisecond)
	if err := db1.closeConnection(); err != nil {
		t.Fatalf("closeConnection() db1 failed: %v", err)
	}

	select {
	case <-done:
		if err2 != nil {
			t.Errorf("openDB() db2 failed: %v", err2)
		}
		if db2 == nil {
			t.Errorf("db2 is nil")
		} else {
			err := db2.closeConnection()
			if err != nil {
				return
			}
		}
	case <-time.After(2 * time.Second):
		t.Errorf("Timeout waiting for concurrent openDB() to complete")
	}
}

// TestCloseIdempotent verifies that calling closeConnection multiple times does not produce an error.
func TestCloseIdempotent(t *testing.T) {
	db, err := openDB("testdb_close", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	if err := db.closeConnection(); err != nil {
		t.Fatalf("First closeConnection() failed: %v", err)
	}
	// Second closeConnection() should be a no-op.
	if err := db.closeConnection(); err != nil {
		t.Errorf("Second closeConnection() returned error: %v", err)
	}
}

// TestDebug calls debugDB to verify no panic occurs.
func TestDebug(t *testing.T) {
	db, err := openDB("testdb_debug", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)
	db.debugDB(true)
	db.debugDB(false)
}

// TestGetNonexistentKey verifies that getJSON returns an error when a key does not exist.
func TestGetNonexistentKey(t *testing.T) {
	db, err := openDB("testdb_nonexistent", false)
	if err != nil {
		t.Fatalf("openDB() failed: %v", err)
	}
	defer func(db *database) {
		err := db.closeConnection()
		if err != nil {

		}
	}(db)

	var result string
	err = db.getJSON("keystore", "nonexistent", &result)
	if err == nil {
		t.Errorf("Expected error for non-existent key, got nil")
	}
}
