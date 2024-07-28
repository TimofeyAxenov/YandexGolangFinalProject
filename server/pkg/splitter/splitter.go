package splitter

import (
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

var TaskExpIds []int

type AllExps struct {
	Expressions []string
}

type PolishConverter struct {
	expression_stack []string
	oper_stack       []string
	rpn_exp          string
	rpn_stack        []string
}

type NewExp struct {
	Expression string `json:"expression"`
}

type ExpID struct {
	Id int `json:"id"`
}

type Exp struct {
	Id         int    `json:"id"`
	Status     string `json:"status"`
	Expression string `json:"expression"`
}

type DBExp struct {
	Id     int    `json:"id"`
	Status string `json:"status"`
	Result int    `json:"result"`
}

type Task struct {
	Id            string `json:"id"`
	Arg1          int    `json:"arg1"`
	Arg2          int    `json:"arg2"`
	Operation     string `json:"operation"`
	OperationTime int64  `json:"operation_time"`
}

var TaskIds []int

type SpecificExp struct {
	Expression string `json:"expression"`
}

type CompletedTask struct {
	Id     int `json:"id"`
	Result int `json:"result"`
}

type SplitExpr struct {
	Id   int    `json:"id"`
	Expr string `json:"expr"`
}

// ConvertToReversePolish перерабатывает выражение в обратную польскую нотацию (2+2*2->[2, 2, 2, *, +]
func ConvertToReversePolishStack(exp string) string {
	var r = &PolishConverter{}
	expression := exp
	var list []string
	tempStr := ""
	isLastCharNumeric := false

	for i := 0; i < len(expression); i++ {
		//get byte as char
		tempChar := fmt.Sprintf("%c", expression[i])

		//r.printDebug(tempChar)
		//check if char is numeric or dot "." / doest check if is "e"
		if IsANumber(tempChar) {
			//if previous char is numeric OR list is empty
			if isLastCharNumeric || len(list) == 0 {
				tempStr = tempStr + tempChar
			} else {
				tempStr = tempStr + tempChar
			}
			isLastCharNumeric = true
		} else {
			if isLastCharNumeric {
				//add number to list
				list = append(list, tempStr)
			}

			tempStr = ""
			//add char to list
			list = append(list, tempChar)

			//set "previous char is numeric" as false
			isLastCharNumeric = false

		}

		//if is the last char of string
		if i == (len(expression) - 1) {
			//check if it is numeric
			if IsANumber(tempChar) {
				//add number to list
				list = append(list, tempStr)
			} else {
				//add char to list
				list = append(list, tempChar)
			}

		}

	}

	/* for i := range list {
		item := list[i]
		r.printDebug(item)
	} */

	r.expression_stack = list
	r.ConvertToReversePolish()
	newexp := r.rpn_exp
	split := strings.Split(newexp, " ")
	out := strings.Join(split, ",")
	return out
}

// MakeTasks проверяет реверсивную польскую нотацию на наличие возможных задач
func MakeTasks(expid int) error {
	q1 := "SELECT splitexp FROM exps WHERE expid = $1"
	rows, err := db.Query(q1, expid)
	if err != nil {
		return err
	}
	defer rows.Close()
	var splitexp string
	rows.Scan(&splitexp)
	exp := strings.Split(splitexp, ",")
	if len(exp) == 1 {
		res, err1 := strconv.Atoi(exp[0])
		if err1 != nil {
			return err1
		}
		FinishExp(expid, res)
		return nil
	}
	for {
		i := 0
		for i = 0; i < len(exp); i++ {
			curr := exp[i]
			if curr == "+" || curr == "-" || curr == "*" || curr == "/" {
				exp, _ = CreateTask(exp, i, expid)
				break
			}
		}
		if i == len(exp) {
			break
		}
	}
	newstr := strings.Join(exp, ",")
	q2 := "UPDATE exps SET splitexp = $1 WHERE expid = $2"
	_, err = db.Exec(q2, newstr, expid)
	if err != nil {
		return err
	}
	return nil
}

// CreateTask проверяет, моно ли создать задачу. Если можно, то создаёт задачу и записывает в базу данных
func CreateTask(exp []string, index int, expid int) ([]string, error) {
	oper := exp[index]
	arg1 := exp[index-2]
	arg2 := exp[index-1]
	num1, err := strconv.Atoi(arg1)
	if err != nil {
		return nil, err
	}
	num2, err := strconv.Atoi(arg2)
	if err != nil {
		return nil, err
	}
	NewTaskID := TaskExpIds[expid-1]
	t := MakeTime(oper)
	taskid := strconv.Itoa(expid) + "." + strconv.Itoa(NewTaskID)
	q := "INSERT INTO exptasks (exptaskid, arg1, oper, arg2, opertime, issent) values ($1, $2, $3, $4, $5, 0)"
	_, err = db.Exec(q, taskid, num1, oper, num2, t)
	if err != nil {
		return nil, err
	}
	exp[index] = "task" + strconv.Itoa(NewTaskID)
	NewTaskID++
	TaskExpIds[expid-1] = NewTaskID
	exp = slices.Delete(exp, index-2, index-1)
	return exp, nil
}

func MakeTime(oper string) int64 {
	var out int64
	switch oper {
	case "+":
		amount := os.Getenv("TIME_ADDITION_MS")
		ms, _ := strconv.Atoi(amount)
		out = int64(ms)
	case "-":
		amount := os.Getenv("TIME_SUBTRACTION_MS")
		ms, _ := strconv.Atoi(amount)
		out = int64(ms)
	case "*":
		amount := os.Getenv("TIME_MULTIPLICATIONS_MS")
		ms, _ := strconv.Atoi(amount)
		out = int64(ms)
	case "/":
		amount := os.Getenv("TIME_DIVISIONS_MS")
		ms, _ := strconv.Atoi(amount)
		out = int64(ms)
	}
	return out
}

// PlaceTask обрабатывает ответ на задачу и размещает его в ОПН вместо изначального условия
func PlaceTask(taskid string, result int) error {
	splitid := strings.Split(taskid, ".")
	expid := splitid[0]
	q1 := "SELECT splitexp FROM exps WHERE expid = $1"
	rows, err := db.Query(q1, expid)
	if err != nil {
		return err
	}
	defer rows.Close()
	var express string
	rows.Scan(&express)
	taskkey := "task" + splitid[1]
	expression := strings.Split(express, ",")
	index := slices.Index(expression, taskkey)
	expression[index] = strconv.Itoa(result)
	newexp := strings.Join(expression, ",")
	q2 := "UPDATE exps SET splitexp = $1 WHERE expid = $2"
	_, err = db.Exec(q2, newexp, expid)
	if err != nil {
		return err
	}
	return nil
}

// FinishExp вызывается, когда на выражение есть ответ
func FinishExp(expid int, result int) {
	q := "UPDATE exps SET result = $1, status = 'решено', splitexp = NULL, WHERE expid = $2"
	db.Exec(q, result, expid)
}

func RecieveDB(DB *sql.DB) {
	db = DB
}

func IsANumber(symb string) bool {
	if symb == "1" || symb == "2" || symb == "3" || symb == "4" || symb == "5" || symb == "6" || symb == "7" || symb == "8" || symb == "9" || symb == "0" {
		return true
	}
	return false
}

func (r *PolishConverter) ConvertToReversePolish() {
	var output string

	first_i := true

	stack := r.expression_stack

	for i := range stack {
		item := stack[i]

		if IsOperator(item) {
			//if stack of operator is empty, just add operator to stack

			if r.GetOperatorStackLength() == 0 || first_i {
				first_i = false
				r.AppendRPNOperatorItem(item)

			} else {

				//When item is "(", should add to stack, and go to next item"
				if item == "(" || item == " " {
					r.AppendRPNOperatorItem(item)

					continue
				}
				if r.GetOperatorStackLength() > 0 && item == ")" {

					for r.GetOperatorStackLength() > 0 && r.GetLastOperatorFromStack() != "(" {
						r.AppendRPNItem(r.GetLastOperatorFromStack())

						//pop from stack
						r.PopOperatorFromStack()
						//r.GetLastOperatorFromStack()

					}
					//WHEN FIND A "(" POP IT, AND GO TO NEXT CHAR OF EXPRESSION
					if r.GetOperatorStackLength() > 0 && r.GetLastOperatorFromStack() == "(" {
						r.PopOperatorFromStack()
					}
					continue
				}

				poped_loop := false
				//check operator precedente, while actual operator is equal or  lower than last operator of stack, add last operator of stack to rpn_expression, should use while() go -> for
				for r.GetOperatorStackLength() > 0 && (r.CheckPrecedence(item) <= r.CheckPrecedence(r.GetLastOperatorFromStack())) {
					r.AppendRPNItem(r.GetLastOperatorFromStack())

					//pop from stack
					r.PopOperatorFromStack()

					poped_loop = true
				}

				if poped_loop {
					r.AppendRPNOperatorItem(item)
					poped_loop = false
				} else if r.GetOperatorStackLength() > 0 && (r.CheckPrecedence(item) > r.CheckPrecedence(r.GetLastOperatorFromStack())) {
					//check operator precedence, if actual operator is bigger than last operator of stack, add actual operator to stack
					r.AppendRPNOperatorItem(item)

				}
			}

		} else {
			//if its not operator, add item to rpn expression
			r.AppendRPNItem(item)

		}
	}

	for r.GetOperatorStackLength() > 0 {

		r.AppendRPNItem(r.GetLastOperatorFromStack())
		//pop from stack
		r.PopOperatorFromStack()

	}
	output = strings.Trim(output, " ")
	output = strings.TrimRight(output, " ")
	r.rpn_exp = output
}

func IsOperator(value string) bool {
	if value == "*" || value == "/" || value == "+" || value == "-" || value == "=" || value == ")" || value == "(" {
		return true
	}
	return false
}

func (r *PolishConverter) GetOperatorStackLength() int {
	return len(r.oper_stack)
}

func (r *PolishConverter) AppendRPNOperatorItem(item string) {
	r.oper_stack = append(r.oper_stack, item)
}

func (r *PolishConverter) GetLastOperatorFromStack() string {
	if len(r.oper_stack) > 0 {
		//r.printDebug(r.oper_stack[len(r.oper_stack)-1])
		return r.oper_stack[len(r.oper_stack)-1]
	}
	return ""
}

func (r *PolishConverter) AppendRPNItem(item string) {
	if item != "(" && item != ")" {
		r.rpn_exp = r.rpn_exp + item + " "
		r.rpn_stack = append(r.rpn_stack, item)
	}
}

func (r *PolishConverter) PopOperatorFromStack() []string {
	if len(r.oper_stack) > 0 {
		r.oper_stack = r.oper_stack[:len(r.oper_stack)-1]
	}

	return r.oper_stack
}

func (r *PolishConverter) CheckPrecedence(item string) int {
	switch item {
	case "*":
		return 30
	case "/":
		return 30
	case "+":
		return 20
	case "-":
		return 20
	}
	return 0
}
