package main

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "strings"
    "time"
)

func main() {
    var (
        lp *LogProcess
        r  Reader
        w  Writer
        rc = make(chan []byte)
        wc = make(chan string)
    )
    r = NewReadFromFile("./access.log")
    w = NewWriteToFluxDB("username@password")
    lp = NewLogProcess(rc, wc, r, w)

    go lp.read.Read(lp.rc)
    go lp.Process()
    go lp.write.Write(lp.wc)

    time.Sleep(30 * time.Second)
}

// 读取接口
type Reader interface {
    Read(rc chan []byte)
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
func (r *ReadFromFile) Read(rc chan []byte) {
    var (
        line   []byte
        file *os.File
        rd   *bufio.Reader
        err  error
    )
    // 打开文件
    if file, err = os.Open(r.path); err != nil {
        panic(fmt.Sprintf("open file error: %s", err.Error()))
    }

    // 循环从文件末尾开始逐行读取文件内容
    if _, err = file.Seek(0, 2); err != nil {
        panic(fmt.Sprintf("read file err: %s", err.Error()))
    }
    rd = bufio.NewReader(file)
    for {
        line, err = rd.ReadBytes('\n')
        if err == io.EOF { // 读取到文件结尾，等待文件中有新的内容写入
            time.Sleep(time.Millisecond * 500) // 休眠500ms
            continue
        } else if err != nil {
            panic(fmt.Sprintf("Read bytes err: %s", err.Error()))
        }
        rc <- line[:len(line)-1] // 模拟往 ReadChan中写入一行数据，删除末尾的换行符
    }
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
    var (
        v string
    )
    for v = range wc {
        fmt.Println(v) // 获取WriteChan的数据直接打印出来
    }
}

// 日志解析结构体
type LogProcess struct {
    rc    chan []byte // 读取channel
    wc    chan string // 写入channel
    read  Reader
    write Writer
}

// LogProcess结构体构造函数
func NewLogProcess(rc chan []byte, wc chan string, read Reader, write Writer) *LogProcess {
    return &LogProcess{
        rc:    rc,
        wc:    wc,
        read:  read,
        write: write,
    }
}

// 解析模块，解析文件内容
func (l *LogProcess) Process() {
    var (
        v []byte
    )
    for v = range l.rc {
        l.wc <- strings.ToUpper(string(v)) // 将ReadChan中的数据获取并转化成大写传递给WriteChan
    }
}
