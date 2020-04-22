package main

import (
    "fmt"
    "strings"
    "time"
)

func main() {
    var (
        lp *LogProcess
        r  Reader
        w  Writer
    )
    r = NewReadFromFile("./tmp/access.log")
    w = NewWriteToFluxDB("username@password")
    lp = NewLogProcess(r, w)

    go lp.read.Read(lp.rc)
    go lp.Process()
    go lp.write.Write(lp.wc)

    time.Sleep(time.Second)
}

// 读取接口
type Reader interface {
    Read(rc chan string)
}

type ReadFromFile struct {
    path string // 文件读取的地址
}

// 构造函数
func NewReadFromFile(path string) *ReadFromFile {
    return &ReadFromFile{
        path: path,
    }
}

// 读取模块
func (r *ReadFromFile) Read(rc chan string) {
    var (
        line = "message"
    )
    rc <- line // 模拟往 ReadChan中写入一行数据
}

// 写入接口
type Writer interface {
    Write(wc chan string)
}

// 写入到fluxDB
type WriteToFluxDB struct {
    influxDBDSN string // influxDB连接地址
}

// 构造函数
func NewWriteToFluxDB(influxDBDNS string) *WriteToFluxDB {
    return &WriteToFluxDB{
        influxDBDSN: influxDBDNS,
    }
}

// 写入模块
func (w *WriteToFluxDB) Write(wc chan string) {
    fmt.Println(<-wc) // 获取WriteChan的数据直接打印出来
}

// 日志解析结构体
type LogProcess struct {
    rc    chan string // 读取channel
    wc    chan string // 写入channel
    read  Reader
    write Writer
}

// LogProcess结构体构造函数
func NewLogProcess(read Reader, write Writer) *LogProcess {
    return &LogProcess{
        rc:    make(chan string),
        wc:    make(chan string),
        read:  read,
        write: write,
    }
}

// 解析模块，解析文件内容
func (l *LogProcess) Process() {
    l.wc <- strings.ToUpper(<-l.rc) // 将ReadChan中的数据获取并转化成大写传递给WriteChan
}