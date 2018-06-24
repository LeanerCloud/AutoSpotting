// Copyright 2017-2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not use this file except in compliance with the License. A copy of the License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

// +build !go1.8

package xray

import (
	"context"
	"database/sql"
)

// Begin starts a transaction.
func (db *DB) Begin(ctx context.Context, opts interface{}) (*Tx, error) {
	tx, err := db.db.Begin()
	return &Tx{db, tx}, err
}

// Prepare creates a prepared statement for later queries or executions.
func (db *DB) Prepare(ctx context.Context, query string) (*Stmt, error) {
	stmt, err := db.db.Prepare(query)
	return &Stmt{db, stmt, query}, err
}

// Ping traces verifying a connection to the database is still alive,
// establishing a connection if necessary and adds corresponding information into subsegment.
func (db *DB) Ping(ctx context.Context) error {
	return Capture(ctx, db.dbname, func(ctx context.Context) error {
		db.populate(ctx, "PING")
		return db.db.Ping()
	})
}

// Exec captures executing a query without returning any rows and
// adds corresponding information into subsegment.
func (db *DB) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var res sql.Result

	err := Capture(ctx, db.dbname, func(ctx context.Context) error {
		db.populate(ctx, query)

		var err error
		res, err = db.db.Exec(query, args...)
		return err
	})

	return res, err
}

// Query captures executing a query that returns rows and adds corresponding information into subsegment.
func (db *DB) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var res *sql.Rows

	err := Capture(ctx, db.dbname, func(ctx context.Context) error {
		db.populate(ctx, query)

		var err error
		res, err = db.db.Query(query, args...)
		return err
	})

	return res, err
}

// QueryRow captures executing a query that is expected to return at most one row
// and adds corresponding information into subsegment.
func (db *DB) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	var res *sql.Row

	Capture(ctx, db.dbname, func(ctx context.Context) error {
		db.populate(ctx, query)

		res = db.db.QueryRow(query, args...)
		return nil
	})

	return res
}

// Exec captures executing a query that doesn't return rows and adds
// corresponding information into subsegment.
func (tx *Tx) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var res sql.Result

	err := Capture(ctx, tx.db.dbname, func(ctx context.Context) error {
		tx.db.populate(ctx, query)

		var err error
		res, err = tx.tx.Exec(query, args...)
		return err
	})

	return res, err
}

// Query captures executing a query that returns rows and adds corresponding information into subsegment.
func (tx *Tx) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var res *sql.Rows

	err := Capture(ctx, tx.db.dbname, func(ctx context.Context) error {
		tx.db.populate(ctx, query)

		var err error
		res, err = tx.tx.Query(query, args...)
		return err
	})

	return res, err
}

// QueryRow captures executing a query that is expected to return at most one row and adds
// corresponding information into subsegment.
func (tx *Tx) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	var res *sql.Row

	Capture(ctx, tx.db.dbname, func(ctx context.Context) error {
		tx.db.populate(ctx, query)

		res = tx.tx.QueryRow(query, args...)
		return nil
	})

	return res
}

// Stmt returns a transaction-specific prepared statement from an existing statement.
func (tx *Tx) Stmt(ctx context.Context, stmt *Stmt) *Stmt {
	return &Stmt{stmt.db, tx.tx.Stmt(stmt.stmt), stmt.query}
}

// Exec captures executing a prepared statement with the given arguments and
// returning a Result summarizing the effect of the statement and adds corresponding
// information into subsegment.
func (stmt *Stmt) Exec(ctx context.Context, args ...interface{}) (sql.Result, error) {
	var res sql.Result

	err := Capture(ctx, stmt.db.dbname, func(ctx context.Context) error {
		stmt.populate(ctx, stmt.query)

		var err error
		res, err = stmt.stmt.Exec(args...)
		return err
	})

	return res, err
}

// Query captures executing a prepared query statement with the given arguments
// and returning the query results as a *Rows and adds corresponding information
// into subsegment.
func (stmt *Stmt) Query(ctx context.Context, args ...interface{}) (*sql.Rows, error) {
	var res *sql.Rows

	err := Capture(ctx, stmt.db.dbname, func(ctx context.Context) error {
		stmt.populate(ctx, stmt.query)

		var err error
		res, err = stmt.stmt.Query(args...)
		return err
	})

	return res, err
}

// QueryRow captures executing a prepared query statement with the given arguments and
// adds corresponding information into subsegment.
func (stmt *Stmt) QueryRow(ctx context.Context, args ...interface{}) *sql.Row {
	var res *sql.Row

	Capture(ctx, stmt.db.dbname, func(ctx context.Context) error {
		stmt.populate(ctx, stmt.query)

		res = stmt.stmt.QueryRow(args...)
		return nil
	})

	return res
}
