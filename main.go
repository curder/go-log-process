package main

import (
    "bufio"
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "github.com/influxdata/influxdb-client-go"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/signal"
    "regexp"
    "strconv"
    "strings"
    "syscall"
    "time"
)

func main() {
    var (
        path      string
        influxDSN string
        lp        *LogProcess
        r         Reader
        w         Writer
        rc        = make(chan []byte, 200)
        wc        = make(chan *Message, 200)
        monitor   *Monitor
        i         int
    )

    flag.StringVar(&path, "path", "./access.log", "read file path")
    flag.StringVar(&influxDSN, "influxDSN", "http://127.0.0.1:9999@uEXWsOHK-zWkTCQtU8dnVHBdfmaptZT3QKy7akghDhPZxnCUOG20CEEtuYD1C0_Tw1TSbihwARl8YdtD1Sa03Q==@my-org@my-bucket", "influx data source")
    flag.Parse()

    r = NewReadFromFile(path)
    w = NewWriteToFluxDB(influxDSN)
    lp = NewLogProcess(rc, wc, r, w)

    go lp.read.Read(lp.rc)
    for i = 0; i < 2; i++ {
        go lp.Process()
    }
    for i = 0; i < 4; i++ {
        go lp.write.Write(lp.wc)
    }

    // 监控模块
    monitor = NewMonitor(time.Now(), SystemInfo{})
    go monitor.start(lp)

    c := make(chan os.Signal)
    signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR1)
    for s := range c {
        switch s {
        case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
            log.Println("capture exit signal:", s)
            os.Exit(1)
        case syscall.SIGUSR1: // 用户自定义信号
            log.Println(lp)
        default:
            log.Println("capture other signal:", s)
        }
    }
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
    // TODO 处理日志切割的问题
    rd = bufio.NewReader(file)
    for {
        line, err = rd.ReadBytes('\n')
        if err == io.EOF { // 读取到文件结尾，等待文件中有新的内容写入
            time.Sleep(time.Millisecond * 500) // 休眠500ms
            continue
        } else if err != nil {
            panic(fmt.Sprintf("Read bytes err: %s", err.Error()))
        }

        TypeMonitorChan <- TypeHandleLine

        rc <- line[:len(line)-1] // 模拟往 ReadChan中写入一行数据，删除末尾的换行符
    }
}

// 写入接口
type Writer interface {
    Write(wc chan *Message)
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
func (w *WriteToFluxDB) Write(wc chan *Message) {
    var (
        influxSlices []string
        err          error
        p            *influxdb2.Point
        v            *Message
    )
    influxSlices = strings.Split(w.influxDBDSN, "@")

    // create new client with default option for server url authenticate by token
    client := influxdb2.NewClient(influxSlices[0], influxSlices[1])
    defer client.Close()
    // user blocking write client for writes to desired bucket
    writeApi := client.WriteApiBlocking(influxSlices[2], influxSlices[3])
    // create point using fluent style
    for v = range wc {
        // Tags: Path, Methods,, Scheme, Status
        // Fields: UpstreamTime, RequestTime, BytesSent
        p = influxdb2.NewPointWithMeasurement("nginx_logs").
            AddTag("Path", v.Path).
            AddTag("Methods", v.Method).
            AddTag("Scheme", v.Scheme).
            AddTag("Status", v.Status).
            AddField("UpstreamTime", v.UpstreamTime).
            AddField("RequestTime", v.RequestTime).
            AddField("BytesSent", v.BytesSent).
            SetTime(v.TimeLocal)
        if err = writeApi.WritePoint(context.Background(), p); err != nil {
            panic(err.Error())
        }

        // fmt.Println(v) // 获取WriteChan的数据直接打印出来
    }
}

// 日志解析结构体
type LogProcess struct {
    rc    chan []byte   // 读取channel
    wc    chan *Message // 写入channel
    read  Reader
    write Writer
}

// LogProcess结构体构造函数
func NewLogProcess(rc chan []byte, wc chan *Message, read Reader, write Writer) *LogProcess {
    return &LogProcess{
        rc:    rc,
        wc:    wc,
        read:  read,
        write: write,
    }
}

type Message struct {
    TimeLocal    time.Time // 发送请求的时间
    BytesSent    int       // 发送字节数
    Path         string    // 请求路径
    Method       string    // 请求方法
    Scheme       string    // 协议
    Status       string    // 状态
    UpstreamTime float64   // 上层处理请求时间，比如说PHP or MySQL处理时间
    RequestTime  float64   // 响应时间
}

// 解析模块，解析文件内容
// 从rc中读取美航日志数据，使用正则提取所需的监控数据(请求路径，状态和方法等)，写入到wc
func (l *LogProcess) Process() {
    var (
        v       []byte
        regex   *regexp.Regexp
        result  []string
        message = &Message{}

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
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("FindStringSubmatch fail: %s", v)
            continue
        }
        if timeLocal, err = time.ParseInLocation("02/Jan/2006:15:04:05 +0000", result[4], loc); err != nil { // 时间
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("ParseInLocation fail: %s, got %s", err.Error(), result[4])
        }
        message.TimeLocal = timeLocal

        if byteSent, err = strconv.Atoi(result[8]); err != nil { // 发送字节数
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("Atoi fail: %s, got %s", err.Error(), result[8])
            continue
        }
        message.BytesSent = byteSent

        splitSlice = strings.Split(result[6], " ")

        if len(splitSlice) != 3 { // 请求方法
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("strings.Split fail %s", result[6])
            continue
        }
        message.Method = splitSlice[0]
        if urlParse, err = url.Parse(splitSlice[1]); err != nil { // 请求地址
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("url parse fail: %s", err.Error())
            continue
        }
        message.Path = urlParse.Path

        message.Scheme = result[5]
        message.Status = result[7]

        if upstreamTime, err = strconv.ParseFloat(result[12], 64); err != nil {
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("upstreamTime ParseFloat fail: %s", err.Error())
            continue
        }
        message.UpstreamTime = upstreamTime
        if requestTime, err = strconv.ParseFloat(result[13], 64); err != nil {
            TypeMonitorChan <- TypeErrorNumber
            log.Printf("requestTime ParseFloat fail: %s", err.Error())
            continue
        }
        message.RequestTime = requestTime

        l.wc <- message // 将ReadChan中的数据获取并转化成大写传递给WriteChan
    }
}

// 系统状态监控
type SystemInfo struct {
    HandleLine   int     `json:"handleLine"`   // 总处理日志行
    Tps          float64 `json:"tps"`          // 系统吞出量
    ReadChanLen  int     `json:"readChanLen"`  // 读Channel 长度
    WriteChanLen int     `json:"writeChanLen"` // 写Channel 长度
    RunTime      string  `json:"runTime"`      // 运行总时间
    ErrorNumber  int     `json:"errNumber"`    // 错误数
}

type TypeMonitor int

// 辅助处理行数和错误行数
const (
    TypeHandleLine TypeMonitor = iota
    TypeErrorNumber
)

var TypeMonitorChan = make(chan TypeMonitor, 200)

type Monitor struct {
    startTime time.Time
    tpsSlice  []int
    data      SystemInfo
}

// 系统监控构造函数
func NewMonitor(startTime time.Time, data SystemInfo) *Monitor {
    return &Monitor{
        startTime: startTime,
        data:      data,
    }
}

func (m *Monitor) start(lp *LogProcess) {
    var (
        n      TypeMonitor
        ticker <-chan time.Time
    )
    // 消费处理行数和错误行数
    go func() {
        for n = range TypeMonitorChan {
            switch n {
            case TypeErrorNumber:
                m.data.ErrorNumber += 1
            case TypeHandleLine:
                m.data.HandleLine += 1
            }
        }
    }()

    ticker = time.Tick(time.Second * 5)

    go func() {
        for {
            <-ticker
            m.tpsSlice = append(m.tpsSlice, m.data.HandleLine)
            if len(m.tpsSlice) > 2 {
                m.tpsSlice = m.tpsSlice[1:]
            }
        }
    }()

    http.HandleFunc("/monitor", func(w http.ResponseWriter, r *http.Request) {
        var (
            result []byte
            err    error
        )
        m.data.RunTime = time.Now().Sub(m.startTime).String()
        m.data.ReadChanLen = len(lp.rc)
        m.data.WriteChanLen = len(lp.wc)

        if len(m.tpsSlice) >= 2 {
            m.data.Tps = float64((m.tpsSlice[1] - m.tpsSlice[0]) / 5)
        }

        if result, err = json.MarshalIndent(m.data, "", "\t"); err != nil {
            fmt.Printf("MarshalIndent failed: %s", err.Error())
            return
        }

        if _, err = io.WriteString(w, string(result)); err != nil {
            fmt.Printf("WriteString failed: %s", err.Error())
            return
        }
    })

    log.Fatal(http.ListenAndServe(":9191", nil))
}
