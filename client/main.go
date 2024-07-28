package main

import (
	pb "agent/proto"
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CompletedTask struct {
	Id     string `json:"id"`
	Result int    `json:"result"`
}

type Task struct {
	Id            string `json:"id"`
	Arg1          int    `json:"arg1"`
	Arg2          int    `json:"arg2"`
	Operation     string `json:"operation"`
	OperationTime int64  `json:"operation_time"`
}

func main() {
	host := "localhost"
	port := "5000"

	addr := fmt.Sprintf("%s:%s", host, port) // используем адрес сервера
	// установим соединение
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer conn.Close()

	if err != nil {
		log.Println("could not connect to grpc server: ", err)
		os.Exit(1)
	}

	grpcClient := pb.NewTaskExchangerClient(conn)

	results := make(chan CompletedTask)
	os.Setenv("COMPUTING_POWER", "5")
	maxg, err := strconv.Atoi(os.Getenv("COMPUTING_POWER"))
	if err != nil {
		panic(err)
	}
	for {
		select {
		case complete := <-results:
			sendtask(grpcClient, complete)
		default:
			curramount := runtime.NumGoroutine()
			if curramount < maxg {
				task := gettask(grpcClient)
				go CountTask(task, results)
			}
		}
	}
}

func CountTask(task Task, res chan CompletedTask) {
	arg1 := task.Arg1
	arg2 := task.Arg2
	oper := task.Operation
	var result int
	switch oper {
	case "+":
		result = arg1 + arg2
	case "-":
		result = arg1 - arg2
	case "*":
		result = arg1 * arg2
	case "/":
		result = arg1 / arg2
	}
	complete := CompletedTask{
		Id:     task.Id,
		Result: result,
	}
	time.Sleep(time.Duration(task.OperationTime) * time.Millisecond)
	res <- complete
}

func gettask(tc pb.TaskExchangerClient) Task {
	var t Task
	task, err := tc.SendTask(context.TODO(), &pb.Null{})
	if err != nil {
		time.Sleep(time.Duration(1) * time.Minute)
		gettask(tc)
	}
	t.Id = task.Taskid
	t.Arg1 = int(task.Arg1)
	t.Arg2 = int(task.Arg2)
	t.Operation = task.Oper
	t.OperationTime = task.Duration
	return t
}

func sendtask(tc pb.TaskExchangerClient, t CompletedTask) {
	res := &pb.TaskResult{}
	res.Taskid = t.Id
	res.Result = int64(t.Result)
	_, err := tc.GetTask(context.TODO(), res)
	if err != nil {
		panic(err)
	}
}
