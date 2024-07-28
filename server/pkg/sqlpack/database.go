package sqlpack

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"orc/pkg/splitter"

	_ "github.com/mattn/go-sqlite3"
)

func MakeDB() *sql.DB {
	db, err := sql.Open("sqlite3", "Storage.db")
	if err != nil {
		panic(err)
	}
	return db
}

func MakeTables(db *sql.DB) error {
	const (
		userExspsTable = `
	CREATE TABLE IF NOT EXISTS exps(
		expid INTEGER,
		expression TEXT,
		splitexp TEXT,
		result INTEGER,
		status TEXT
	)
	`
		expsTasksTable = `
	CREATE TABLE IF NOT EXISTS exptasks(
		exptaskid TEXT,
		arg1 INTEGER,
		oper TEXT,
		arg2 INTEGER,
		taskresult INTEGER,
		opertime INTEGER,
		issent BIT
	)
		`
	)

	if _, err := db.Exec(userExspsTable); err != nil {
		return err
	}
	if _, err := db.Exec(expsTasksTable); err != nil {
		return err
	}

	return nil
}

func GetExps(db *sql.DB) ([]string, error) {
	exp := splitter.DBExp{}
	out := make([]string, 0)
	outb := make([]byte, 0)
	var b []byte
	q := "SELECT expid, status FROM exps"
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var expid int
		var res int
		var status string
		err := rows.Scan(&expid, &status)
		if err != nil {
			return nil, err
		}
		exp.Id = expid
		if status == "решается" {
			res = 0
		} else {
			q1 := "SELECT result FROM exps WHERE expid = $1"
			err = db.QueryRow(q1, expid).Scan(&res)
			if err != nil {
				return nil, err
			}
		}
		exp.Result = res
		exp.Status = status
		b, _ = json.Marshal(exp)
		outb = append(outb, b...)
		out = append(out, string(outb))
	}
	return out, nil
}

func GetExp(db *sql.DB, expid int) (string, error) {
	q1 := "SELECT EXISTS (SELECT expression FROM exps WHERE expid = $1)"
	var exists bool
	err := db.QueryRow(q1, expid).Scan(&exists)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("ErrNotFound")
	}
	exp := splitter.DBExp{}
	q := "SELECT status FROM exps WHERE expid = $1"
	var res int
	var status string
	err = db.QueryRow(q, expid).Scan(&status)
	if err != nil {
		return "", err
	}
	exp.Id = expid
	if status == "решается" {
		res = 0
	} else {
		q1 := "SELECT result FROM exps WHERE expid = $1"
		err = db.QueryRow(q1, expid).Scan(&res)
		if err != nil {
			return "", err
		}
	}
	exp.Result = res
	exp.Status = status
	out, _ := json.Marshal(exp)
	return string(out), nil
}

func CheckNotSplit(db *sql.DB) error {
	q := "SELECT expression, expid FROM exps WHERE splitexp IS NULL AND status = 'решается'"
	rows, err := db.Query(q)
	if err != nil {
		return err
	}

	for rows.Next() {
		var exp string
		var expid int
		err := rows.Scan(&exp, &expid)
		if err != nil {
			return err
		}
		splitexp := splitter.ConvertToReversePolishStack(exp)
		q1 := "UPDATE exps SET splitexp = $1 WHERE expression = $2"
		db.Exec(q1, splitexp, exp)
		splitter.MakeTasks(expid)
	}
	return nil
}

func ReadTask(db *sql.DB) (splitter.Task, error) {
	var task splitter.Task
	q := "SELECT exptaskid, arg1, oper, arg2, opertime FROM exptasks WHERE issent = 0"
	rows, err := db.Query(q)
	if err != nil {
		return task, err
	}
	var id string
	var arg1 int
	var oper string
	var arg2 int
	var opertime int64
	rows.Next()
	err = rows.Err()
	if err != nil {
		return task, err
	}
	err = rows.Scan(&id, &arg1, &oper, &arg2, &opertime)
	if err != nil {
		return task, err
	}
	task.Id = id
	task.Arg1 = arg1
	task.Arg2 = arg2
	task.Operation = oper
	task.OperationTime = opertime
	return task, nil
}
