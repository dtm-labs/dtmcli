package dtmcli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// P2E panic to error
func P2E(perr *error) {
	if x := recover(); x != nil {
		if e, ok := x.(error); ok {
			*perr = e
		} else {
			panic(x)
		}
	}
}

// E2P error to panic
func E2P(err error) {
	if err != nil {
		panic(err)
	}
}

// CatchP catch panic to error
func CatchP(f func()) (rerr error) {
	defer P2E(&rerr)
	f()
	return nil
}

// PanicIf name is clear
func PanicIf(cond bool, err error) {
	if cond {
		panic(err)
	}
}

// MustAtoi 走must逻辑
func MustAtoi(s string) int {
	r, err := strconv.Atoi(s)
	if err != nil {
		E2P(errors.New("convert to int error: " + s))
	}
	return r
}

// OrString return the first not empty string
func OrString(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// If ternary operator
func If(condition bool, trueObj interface{}, falseObj interface{}) interface{} {
	if condition {
		return trueObj
	}
	return falseObj
}

// MustMarshal checked version for marshal
func MustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	E2P(err)
	return b
}

// MustMarshalString string version of MustMarshal
func MustMarshalString(v interface{}) string {
	return string(MustMarshal(v))
}

// MustUnmarshal checked version for unmarshal
func MustUnmarshal(b []byte, obj interface{}) {
	err := json.Unmarshal(b, obj)
	E2P(err)
}

// MustUnmarshalString string version of MustUnmarshal
func MustUnmarshalString(s string, obj interface{}) {
	MustUnmarshal([]byte(s), obj)
}

// MustRemarshal marshal and unmarshal, and check error
func MustRemarshal(from interface{}, to interface{}) {
	b, err := json.Marshal(from)
	E2P(err)
	err = json.Unmarshal(b, to)
	E2P(err)
}

// Logf 输出日志
func Logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	n := time.Now()
	ts := fmt.Sprintf("%s.%03d", n.Format("2006-01-02 15:04:05"), n.Nanosecond()/1000000)
	var file string
	var line int
	for i := 1; ; i++ {
		_, file, line, _ = runtime.Caller(i)
		if strings.Contains(file, "dtm") {
			break
		}
	}
	fmt.Printf("%s %s:%d %s\n", ts, path.Base(file), line, msg)
}

// LogRedf 采用红色打印错误类信息
func LogRedf(fmt string, args ...interface{}) {
	Logf("\x1b[31m\n"+fmt+"\x1b[0m\n", args...)
}

// FatalExitFunc Fatal退出函数，测试时被替换
var FatalExitFunc = func() { os.Exit(1) }

// LogFatalf 采用红色打印错误类信息， 并退出
func LogFatalf(fmt string, args ...interface{}) {
	Logf("\x1b[31m\n"+fmt+"\x1b[0m\n", args...)
	FatalExitFunc()
}

// LogIfFatalf 采用红色打印错误类信息， 并退出
func LogIfFatalf(condition bool, fmt string, args ...interface{}) {
	if condition {
		LogFatalf(fmt, args...)
	}
}

// FatalIfError 采用红色打印错误类信息， 并退出
func FatalIfError(err error) {
	LogIfFatalf(err != nil, "Fatal error: %v", err)
}

// RestyClient the resty object
var RestyClient = resty.New()

func init() {
	// RestyClient.SetTimeout(3 * time.Second)
	// RestyClient.SetRetryCount(2)
	// RestyClient.SetRetryWaitTime(1 * time.Second)
	RestyClient.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		r.URL = MayReplaceLocalhost(r.URL)
		Logf("requesting: %s %s %v %v", r.Method, r.URL, r.Body, r.QueryParam)
		return nil
	})
	RestyClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		r := resp.Request
		Logf("requested: %s %s %s", r.Method, r.URL, resp.String())
		return nil
	})
}

// GetFuncName get current call func name
func GetFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

// MayReplaceLocalhost when run in docker compose, change localhost to host.docker.internal for accessing host network
func MayReplaceLocalhost(host string) string {
	if os.Getenv("IS_DOCKER") != "" {
		return strings.Replace(host, "localhost", "host.docker.internal", 1)
	}
	return host
}

var sqlDbs = map[string]*sql.DB{}

// SdbGet get pooled sql.DB
func SdbGet(conf map[string]string) (*sql.DB, error) {
	dsn := GetDsn(conf)
	if sqlDbs[dsn] == nil {
		db, err := SdbAlone(conf)
		if err != nil {
			return nil, err
		}
		sqlDbs[dsn] = db
	}
	return sqlDbs[dsn], nil
}

// SdbAlone get a standalone db connection
func SdbAlone(conf map[string]string) (*sql.DB, error) {
	dsn := GetDsn(conf)
	Logf("opening alone %s: %s", conf["driver"], strings.Replace(dsn, conf["password"], "****", 1))
	return sql.Open(conf["driver"], dsn)
}

// SdbExec use raw db to exec
func SdbExec(db *sql.DB, sql string, values ...interface{}) (affected int64, rerr error) {
	r, rerr := db.Exec(sql, values...)
	if rerr == nil {
		affected, rerr = r.RowsAffected()
		Logf("affected: %d for %s %v", affected, sql, values)
	} else {
		LogRedf("exec error: %v for %s %v", rerr, sql, values)
	}
	return
}

// StxExec use raw tx to exec
func StxExec(tx *sql.Tx, sql string, values ...interface{}) (affected int64, rerr error) {
	r, rerr := tx.Exec(sql, values...)
	if rerr == nil {
		affected, rerr = r.RowsAffected()
		Logf("affected: %d for %s %v", affected, sql, values)
	} else {
		LogRedf("exec error: %v for %s %v", rerr, sql, values)
	}
	return
}

// StxQueryRow use raw tx to query row
func StxQueryRow(tx *sql.Tx, query string, args ...interface{}) *sql.Row {
	Logf("querying: %s %v", query, args)
	return tx.QueryRow(query, args...)
}

// GetDsn get dsn from map config
func GetDsn(conf map[string]string) string {
	conf["host"] = MayReplaceLocalhost(conf["host"])
	driver := conf["driver"]
	dsn := MS{
		"mysql": fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
			conf["user"], conf["password"], conf["host"], conf["port"], conf["database"]),
		"postgres": fmt.Sprintf("host=%s user=%s password=%s dbname='%s' port=%s sslmode=disable TimeZone=Asia/Shanghai",
			conf["host"], conf["user"], conf["password"], conf["database"], conf["port"]),
	}[driver]
	PanicIf(dsn == "", fmt.Errorf("unknow driver: %s", driver))
	return dsn
}

// CheckResponse 检查Response，返回错误
func CheckResponse(resp *resty.Response, err error) error {
	if err == nil && resp != nil {
		if resp.IsError() {
			return errors.New(resp.String())
		} else if strings.Contains(resp.String(), "FAILURE") {
			return ErrFailure
		}
	}
	return err
}

// CheckResult 检查Result，返回错误
func CheckResult(res interface{}, err error) error {
	resp, ok := res.(*resty.Response)
	if ok {
		return CheckResponse(resp, err)
	}
	if res != nil {
		str := MustMarshalString(res)
		if strings.Contains(str, "FAILURE") {
			return ErrFailure
		} else if strings.Contains(str, "PENDING") {
			return ErrPending
		}
	}
	return err
}
