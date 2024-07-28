package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"orc/pkg/splitter"
	"orc/pkg/sqlpack"
	pb "orc/proto"
	"os"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
)

var db *sql.DB

type TaskServer struct {
	pb.TaskExchangerServer
}

func newTaskServer() *TaskServer {
	return &TaskServer{}
}

func (ts *TaskServer) SendTask(context.Context, *pb.Null) (*pb.Task, error) {
	task := &pb.Task{}
	out, err := sqlpack.ReadTask(db)
	if err != nil {
		return nil, fmt.Errorf("no tasks")
	}
	task.Taskid = out.Id
	task.Arg1 = int64(out.Arg1)
	task.Arg2 = int64(out.Arg2)
	task.Oper = out.Operation
	task.Duration = out.OperationTime
	q := "UPDATE exptasks SET issent = 1 WHERE taskid = $1"
	db.Exec(q, out.Id)
	return task, nil
}

func (ts *TaskServer) GetTask(ctx context.Context, res *pb.TaskResult) (*pb.Null, error) {
	id := res.Taskid
	result := res.Result
	q := "UPDATE exptasks SET taskresult = $1 WHERE exptaskid = $2"
	_, err := db.Exec(q, result, id)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func main() {
	db = sqlpack.MakeDB()
	err := sqlpack.MakeTables(db)
	if err != nil {
		panic(err)
	}
	splitter.RecieveDB(db)
	sqlpack.CheckNotSplit(db)
	go StartGrpcServer()

	e := echo.New()
	e.Use(middleware.Logger())
	//Read a new expression
	e.POST("/api/v1/calculate", func(c echo.Context) error {
		decoder := json.NewDecoder(c.Request().Body)
		newReadExp := splitter.NewExp{}
		newExp := splitter.Exp{}
		err := decoder.Decode(&newReadExp)
		if err != nil {
			log.Println("failed to decode")
			return echo.NewHTTPError(500, err.Error())
		}
		var expid int
		q2 := "SELECT EXISTS (SELECT * FROM exps)"
		var check bool
		err = db.QueryRow(q2).Scan(&check)
		if err != nil {
			log.Println("error while checking exps")
			return echo.NewHTTPError(500, err.Error())
		}
		if !check {
			expid = 1
		} else {
			q1 := "SELECT expid FROM exps ORDER BY expid DESC LIMIT 1"
			err = db.QueryRow(q1).Scan(&expid)
			if err != nil {
				log.Println("failed to get prev id")
			}
			expid++
		}
		exp := newReadExp.Expression
		if strings.Contains(exp, "^") {
			return echo.NewHTTPError(422, "bad expression")
		}
		newExp.Id = expid
		newExp.Expression = exp
		splitexp := splitter.ConvertToReversePolishStack(exp)
		q := "INSERT INTO exps (expid, expression, status, splitexp) VALUES ($1, $2, 'решается', $3)"
		_, err = db.Exec(q, newExp.Id, newExp.Expression, splitexp)
		if err != nil {
			log.Println("insert error")
			return echo.NewHTTPError(500, err.Error())
		}
		splitter.MakeTasks(expid)
		uniqid := splitter.ExpID{Id: expid}
		idjson, err := json.Marshal(uniqid)
		if err != nil {
			log.Println("marshal fail")
			return echo.NewHTTPError(500, err.Error())
		}
		return c.String(201, string(idjson))
	})
	// Write all expressions
	e.GET("/api/v1/expressions/all", func(c echo.Context) error {
		exps, err := sqlpack.GetExps(db)
		if err != nil {
			return echo.NewHTTPError(500, err.Error())
		}
		writtenexps := splitter.AllExps{
			Expressions: exps,
		}
		staskjson, err := json.Marshal(writtenexps)
		if err != nil {
			return echo.NewHTTPError(500, err.Error())
		}
		return c.String(200, string(staskjson))
	})
	// Write a specific expression
	e.GET("/api/v1/expressions/:id", func(c echo.Context) error {
		requestedid := c.Param("id")
		expid, err := strconv.Atoi(requestedid)
		if err != nil {
			return echo.NewHTTPError(500, err.Error())
		}
		gotexp, err := sqlpack.GetExp(db, expid)
		if err != nil {
			switch err.Error() {
			case "ErrNotFound":
				return echo.NewHTTPError(404, err.Error())
			default:
				return echo.NewHTTPError(500, err.Error())
			}
		}
		gotexpstruct := splitter.SpecificExp{Expression: gotexp}
		staskjson, err := json.Marshal(gotexpstruct)
		if err != nil {
			return echo.NewHTTPError(500, err.Error())
		}
		return c.String(200, string(staskjson))
	})
	if err := e.Start(":8080"); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func StartGrpcServer() {
	host := "localhost"
	port := "5000"

	addr := fmt.Sprintf("%s:%s", host, port)

	lis, err := net.Listen("tcp", addr)

	if err != nil {
		log.Println("failed to start grpc listener: ", err)
		os.Exit(1)
	}

	log.Println("grpc listener started")

	grpcServer := grpc.NewServer()

	ServiceServer := newTaskServer()

	pb.RegisterTaskExchangerServer(grpcServer, ServiceServer)

	if err := grpcServer.Serve(lis); err != nil {
		log.Println("error serving grpc: ", err)
		os.Exit(1)
	}
}
