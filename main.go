package main

import (
    "fmt"
    "strings"
    "time"
)

func main() {
    var (
        lp *LogProcess
    )
    lp = NewLogProcess("/tmp/access.log", "username@password")

    go lp.ReadFromFile()
    go lp.Process()
    go lp.WriteToFluxDB()

    time.Sleep(time.Second)
}

// 日志解析结构体
type LogProcess struct {
    ReadChan    chan string // 读取channel
    WriteChan   chan string // 写入channel
    Path        string      // 读取文件的路径
    InfluxDBDSN string      // influxDB连接地址
}

// LogProcess结构体构造函数
func NewLogProcess(path, influxDBDNS string) *LogProcess {
    return &LogProcess{
        ReadChan:    make(chan string),
        WriteChan:   make(chan string),
        Path:        path,
        InfluxDBDSN: influxDBDNS,
    }
}

// 读取模块，从文件中读取
func (l *LogProcess) ReadFromFile() {
    var (
        line string
    )
    line = "message"
    // 模拟往 ReadChan中写入一行数据
    l.ReadChan <- line
}

// 解析模块，解析文件内容
func (l *LogProcess) Process() {
    // 将ReadChan中的数据获取并转化成大写传递给WriteChan
    l.WriteChan <- strings.ToUpper(<-l.ReadChan)
}

// 写入模块，将解析好的内容写入到fluxDB
func (l *LogProcess) WriteToFluxDB() {
    fmt.Println(<-l.WriteChan) // 获取WriteChan的数据直接打印出来
}
