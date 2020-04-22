package main

import (
    "bufio"
    "fmt"
    "io"
    "log"
    "net/url"
    "os"
    "regexp"
    "strconv"
    "strings"
    "time"
)

func main() {
    var (
        lp *LogProcess
        r  Reader
        w  Writer
        rc = make(chan []byte)
        wc = make(chan Message)
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
        line []byte
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
    Write(wc chan Message)
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
func (w *WriteToFluxDB) Write(wc chan Message) {
    var (
        v Message
    )
    for v = range wc {
        fmt.Println(v) // 获取WriteChan的数据直接打印出来
    }
}

// 日志解析结构体
type LogProcess struct {
    rc    chan []byte  // 读取channel
    wc    chan Message // 写入channel
    read  Reader
    write Writer
}

// LogProcess结构体构造函数
func NewLogProcess(rc chan []byte, wc chan Message, read Reader, write Writer) *LogProcess {
    return &LogProcess{
        rc:    rc,
        wc:    wc,
        read:  read,
        write: write,
    }
}

type Message struct {
    TimeLocal    time.Time
    BytesSent    int
    Path         string
    Method       string
    Scheme       string
    Status       string
    UpstreamTime float64
    RequestTime  float64
}

// 解析模块，解析文件内容
// 从rc中读取美航日志数据，使用正则提取所需的监控数据(请求路径，状态和方法等)，写入到wc
func (l *LogProcess) Process() {
    var (
        v       []byte
        regex   *regexp.Regexp
        result  []string
        message = Message{}

        loc, _    = time.LoadLocation("Asia/Shanghai")
        timeLocal time.Time
        err       error

        byteSent   int
        splitSlice []string

        urlParse *url.URL

        upstreamTime float64
        requestTime  float64
    )

    //     log_format  main  '$remote_addr - $http_x_forwarded_for - $remote_user [$time_local] $scheme "$request"'
    //                       '$status $body_bytes_sent "$http_referer"'
    //                       '"$http_user_agent" "$gzip_ratio"'
    //                       '$upstream_response_time $request_time';

    // 172.16.11.77 - - [14/Apr/2020:11:10:52 +0800] "GET /js/app.js?id=4ed2936f15dc73100fbe HTTP/2.0" 200 125250 "https://d1.topcounsel.cn/" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.163 Safari/537.36" "-"
    // ([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)
    regex = regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)`)

    for v = range l.rc {
        result = regex.FindStringSubmatch(string(v))

        if len(result) != 14 {
            log.Printf("FindStringSubmatch fail: %s", v)
            continue
        }
        if timeLocal, err = time.ParseInLocation("02/Jan/2006:15:04:05 +0000", result[4], loc); err != nil { // 时间
            log.Printf("ParseInLocation fail: %s, got %s", err.Error(), result[4])
        }
        message.TimeLocal = timeLocal

        if byteSent, err = strconv.Atoi(result[8]); err != nil { // 发送字节数
            log.Printf("Atoi fail: %s, got %s", err.Error(), result[8])
            continue
        }
        message.BytesSent = byteSent

        splitSlice = strings.Split(result[6], " ")

        if len(splitSlice) != 3 { // 请求方法
            log.Printf("strings.Split fail %s", result[6])
            continue
        }
        message.Method = splitSlice[0]
        if urlParse, err = url.Parse(splitSlice[1]); err != nil { // 请求地址
            log.Printf("url parse fail: %s", err.Error())
            continue
        }
        message.Path = urlParse.Path

        message.Scheme = result[5]
        message.Status = result[7]
        
        if upstreamTime, err = strconv.ParseFloat(result[12], 64); err != nil {
            log.Printf("upstreamTime ParseFloat fail: %s", err.Error())
            continue
        }
        message.UpstreamTime = upstreamTime
        if requestTime, err = strconv.ParseFloat(result[13], 64); err != nil {
            log.Printf("requestTime ParseFloat fail: %s", err.Error())
            continue
        }
        message.RequestTime = requestTime

        l.wc <- message // 将ReadChan中的数据获取并转化成大写传递给WriteChan
    }
}
