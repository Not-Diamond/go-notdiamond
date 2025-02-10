package notdiamond

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mr-tron/base58/base58"
	"github.com/pkg/errors"
	"github.com/schollz/sqlite3dump"
)

// DataFolder is the directory where all SQLite3 databases are stored.
// It should be set to an absolute path in production.
var DataFolder = "."

// Database represents a SQLite database instance.
type Database struct {
	Schema   string  // Full file path to the SQLite database.
	db       *sql.DB // Underlying database connection.
	logger   *logrus.Logger
	isClosed bool
}

// DatabaseLock is used to prevent concurrent access to the same database file.
type DatabaseLock struct {
	Locked map[string]bool
	sync.RWMutex
}

var databaseLock *DatabaseLock

func init() {
	// Initialize the database lock map.
	databaseLock = &DatabaseLock{
		Locked: make(map[string]bool),
	}
}

// MakeTables creates a table with the given name and columns.
// If timeSeries is true, an auto-increment primary key and a timestamp column are added.
func (d *Database) MakeTables(timeSeries bool, tableName string, columns map[string]string) error {
	var sqlStmt string
	if timeSeries {
		sqlStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME`, tableName)
	} else {
		sqlStmt = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			key TEXT NOT NULL PRIMARY KEY`, tableName)
	}
	for col, colType := range columns {
		sqlStmt += fmt.Sprintf(", %s %s", col, colType)
	}
	sqlStmt += ");"

	if _, err := d.db.Exec(sqlStmt); err != nil {
		err = errors.Wrap(err, "MakeTables Exec")
		Error(err)
		return err
	}

	// Create an index on the primary lookup column.
	if timeSeries {
		sqlStmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_idx ON %s(timestamp);`, tableName, tableName)
	} else {
		sqlStmt = fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s_idx ON %s(key);`, tableName, tableName)
	}
	if _, err := d.db.Exec(sqlStmt); err != nil {
		err = errors.Wrap(err, "MakeTables Index")
		Error(err)
		return err
	}

	return nil
}

// Columns returns the list of column names for the specified table.
func (d *Database) Columns(table string) ([]string, error) {
	rows, err := d.db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT 1", table))
	if err != nil {
		return nil, errors.Wrap(err, "Columns Query")
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			Error(err)
		}
	}(rows)

	cols, err := rows.Columns()
	if err != nil {
		return nil, errors.Wrap(err, "Columns Scan")
	}
	return cols, nil
}

// Get retrieves the JSON value associated with the specified key from the table,
// unmarshalling it into the provided interface.
func (d *Database) Get(tableName, key string, v interface{}) error {
	query := fmt.Sprintf("SELECT value FROM %s WHERE key = ?", tableName)
	stmt, err := d.db.Prepare(query)
	if err != nil {
		return errors.Wrap(err, "Get Prepare")
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			Error(err)
		}
	}(stmt)

	var result string
	if err := stmt.QueryRow(key).Scan(&result); err != nil {
		return errors.Wrap(err, "Get QueryRow")
	}

	if err := json.Unmarshal([]byte(result), v); err != nil {
		return errors.Wrap(err, "Get Unmarshal")
	}
	return nil
}

// Set inserts or updates the given key-value pair in the specified table.
// The value is stored as a JSON string.
func (d *Database) Set(tableName, key string, value interface{}) error {
	b, err := json.Marshal(value)
	if err != nil {
		return errors.Wrap(err, "Set Marshal")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Set Begin")
	}
	stmt, err := tx.Prepare(fmt.Sprintf("INSERT OR REPLACE INTO %s(key, value) VALUES (?, ?)", tableName))
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "Set Prepare")
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			Error(err)
		}
	}(stmt)

	if _, err := stmt.Exec(key, string(b)); err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "Set Exec")
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Set Commit")
	}
	return nil
}

// Delete removes the record identified by key from the specified table.
func (d *Database) Delete(tableName, key string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Delete Begin")
	}
	stmt, err := tx.Prepare(fmt.Sprintf("DELETE FROM %s WHERE key = ?", tableName))
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "Delete Prepare")
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			Error(err)
		}
	}(stmt)

	if _, err := stmt.Exec(key); err != nil {
		err := tx.Rollback()
		if err != nil {
			return err
		}
		return errors.Wrap(err, "Delete Exec")
	}
	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Delete Commit")
	}
	return nil
}

// Dump outputs a complete SQL dump of the database.
func (d *Database) Dump() (string, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if err := sqlite3dump.Dump(d.Schema, writer); err != nil {
		return "", errors.Wrap(err, "Dump sqlite3dump")
	}
	if err := writer.Flush(); err != nil {
		return "", errors.Wrap(err, "Dump Flush")
	}
	return buf.String(), nil
}

// Exists checks if a database file for the given name exists in the DataFolder.
// Non-readOnly databases are named using a base58 encoding.
func Exists(name string) error {
	name = strings.TrimSpace(name)
	fileName := filepath.Join(DataFolder, base58.FastBase58Encoding([]byte(name))+".sqlite3.db")
	if _, err := os.Stat(fileName); err != nil {
		return errors.Errorf("database '%s' does not exist", fileName)
	}
	return nil
}

// Drop closes and removes the underlying database file.
func (d *Database) Drop() error {
	if err := d.Close(); err != nil {
		return errors.Wrap(err, "Drop Close")
	}
	Debug("Deleting database file: %s", d.Schema)
	if err := os.Remove(d.Schema); err != nil {
		return errors.Wrap(err, "Drop Remove")
	}
	return nil
}

// Open opens a SQLite3 database with the specified name.
// If readOnly is false, the database file is created (using a base58 encoded name) if it does not exist.
func Open(name string, readOnly bool) (*Database, error) {
	name = strings.TrimSpace(name)
	d := &Database{
		Schema: name,
		logger: Log, // Ensure your logger provides this constructor.
	}

	if readOnly {
		d.Schema = filepath.Join(DataFolder, name)
	} else {
		encodedName := base58.FastBase58Encoding([]byte(name))
		d.Schema = filepath.Join(DataFolder, encodedName+".sqlite3.db")
	}

	// Determine if this is a new (non-readOnly) database.
	newDatabase := false
	if !readOnly {
		if _, err := os.Stat(d.Schema); os.IsNotExist(err) {
			newDatabase = true
		}
	}

	// Acquire a lock for the database file.
	for {
		databaseLock.Lock()
		if _, locked := databaseLock.Locked[d.Schema]; !locked {
			databaseLock.Locked[d.Schema] = true
			databaseLock.Unlock()
			break
		}
		databaseLock.Unlock()
		time.Sleep(10 * time.Millisecond)
	}

	// Open the SQLite3 connection.
	var err error
	d.db, err = sql.Open("sqlite3", d.Schema)
	if err != nil {
		return nil, errors.Wrap(err, "Open sql.Open")
	}

	// If new, create a default table.
	if newDatabase {
		if err := d.MakeTables(false, "keystore", map[string]string{"value": "TEXT"}); err != nil {
			return nil, errors.Wrap(err, "Open MakeTables")
		}
		Debug("Created initial keystore table in new database")
	}

	return d, nil
}

// Debug sets the logger's level based on the debugMode flag.
func (d *Database) Debug(debugMode bool) {
	if debugMode {
		SetLevel("debug")
	} else {
		SetLevel("info")
	}
}

// Close terminates the database connection and releases the file lock.
func (d *Database) Close() error {
	if d.isClosed {
		return nil
	}
	if err := d.db.Close(); err != nil {
		Error(err)
		return errors.Wrap(err, "Close db.Close")
	}
	databaseLock.Lock()
	delete(databaseLock.Locked, d.Schema)
	databaseLock.Unlock()
	d.isClosed = true
	return nil
}

// Query executes a query that returns rows (e.g. SELECT).
func (d *Database) Query(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "Query")
	}
	return rows, nil
}

// Exec executes a query without returning rows (e.g. INSERT, UPDATE, DELETE).
func (d *Database) Exec(query string, args ...interface{}) error {
	if _, err := d.db.Exec(query, args...); err != nil {
		return errors.Wrap(err, "Exec")
	}
	return nil
}
