package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
	"fmt"
	"log"
	"sync"
	//"crypto/md5"
	"flag"
	"strings"
)

var (
	// 最大线程数量
	MaxThread = 150
	// 缓冲区大小
	CacheSize = 8192

	serverAddr = "10.12.212.122:7777"
   
    requestFile = "v1.data"

    //下载的文件的存储路径和名字
    storeFile = "/data/tegcode/team58/data_phase_two.dat"

    proxyAddr1 = "172.16.9.141:8050"
    proxyAddr2 = "172.16.9.141:8090"
    proxyAddr3 = "172.16.9.141:8070"
  
    proxyNum = 3
    proxyAddrs = ""
    proxyAddr []string
    //m = new(sync.RWMutex)
)

type Status struct {
	Downloaded int64
	Speeds     int64
}

type Block struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
}

type FileDl struct {
	Url  string   // 下载地址
	Size int64    // 文件大小
	File *os.File // 要写入的文件

	BlockList []Block // 用于记录未下载的文件块起始位置

	onStart  func()
	onPause  func()
	onResume func()
	onDelete func()
	onFinish func()
	onError  func(int, error)

	paused bool
	status Status
	exited bool
}

// 如果 size <= 0 则自动获取文件大小
func NewFileDl(url string, file *os.File, size int64) (*FileDl, error) {
	if size <= 0 {
		// 获取文件信息
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		size = resp.ContentLength
	}

	f := &FileDl{
		Url:  url,
		Size: size,
		File: file,
	}

	return f, nil
}


// 开始下载
func (f *FileDl) Start() {
	go func() {
		if f.Size <= 0 {
			f.BlockList = append(f.BlockList, Block{0, -1})
		} else {
			blockSize := f.Size / int64(MaxThread)
			var begin int64
			// 数据平均分配给各个线程
			for i := 0; i < MaxThread; i++ {
				var end = (int64(i) + 1) * blockSize
				f.BlockList = append(f.BlockList, Block{begin, end})
				begin = end + 1
			}
			// 将余出数据分配给最后一个线程
			f.BlockList[MaxThread-1].End += f.Size - f.BlockList[MaxThread-1].End


		}

		f.touch(f.onStart)
		// 开始下载
		err := f.download()
		if err != nil {
			f.touchOnError(0, err)
			return
		}

	}()
}

func (f *FileDl) download() error {
	f.startGetSpeeds()

	ok := make(chan bool, MaxThread)
	
	for i := range f.BlockList {
		go func(id int) {
			defer func() {
				ok <- true
			}()

			for {
				
				err := f.downloadBlock(id)
				if err != nil {
					f.touchOnError(0, err)
					// 重新下载
					continue
				}
				break
			}
		}(i)
	}

	for i := 0; i < MaxThread; i++ {
		<-ok
	}
	// 检查是否为暂停导致的“下载完成”
	if f.paused {
		f.touch(f.onPause)
		return nil
	}
	f.paused = true
	f.touch(f.onFinish)
    
	return nil
}

// 文件块下载器
// 根据线程ID获取下载块的起始位置
func (f *FileDl) downloadBlock(id int) error {
	var newUrl string
	if proxyNum != 0{
		//将数据依分发给代理
		newUrl = "http://"+proxyAddr[id%proxyNum]+"/"+requestFile
	}else{
		newUrl = "http://"+serverAddr+"/"+requestFile

	}
	
	request, err := http.NewRequest("GET", newUrl, nil)
	//request, err := http.NewRequest("GET", f.Url, nil)
	if err != nil {
		return err
	}

	begin := f.BlockList[id].Begin
	end := f.BlockList[id].End
	if end != -1 {
		request.Header.Set(
			"Range",
			"bytes="+strconv.FormatInt(begin, 10)+"-"+strconv.FormatInt(end, 10),
		)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var buf = make([]byte, CacheSize)
	for {
		if f.paused == true {
			// 下载暂停
			return nil
		}

		n, e := resp.Body.Read(buf)

		bufSize := int64(len(buf[:n]))
		if end != -1 {
			// 检查下载的大小是否超出需要下载的大小
			// 这里End+1是因为http的Range的end是包括在需要下载的数据内的
			// 比如 0-1 的长度其实是2，所以这里end需要+1
			needSize := f.BlockList[id].End + 1 - f.BlockList[id].Begin
			if bufSize > needSize {
				// 数据大小不正常
				// 一般是因为网络环境不好导致
				// 比如用中国电信下载国外文件
				// 设置数据大小来去掉多余数据
				// 并结束这个线程的下载
				bufSize = needSize
				n = int(needSize)
				e = io.EOF
			}
		}
		// 将缓冲数据写入硬盘
		//m.Lock()
		f.File.WriteAt(buf[:n], f.BlockList[id].Begin)
		//m.Unlock()
		
	

		// 更新已下载大小
		f.status.Downloaded += bufSize
		f.BlockList[id].Begin += bufSize

		if e != nil {
			if e == io.EOF {
				// 数据已经下载完毕
				return nil
			}
			return e
		}
	}

	return nil
}

func (f *FileDl) startGetSpeeds() {
	go func() {
		var old = f.status.Downloaded
		for {
			if f.exited{
				return
			}
			if f.paused {
				f.status.Speeds = 0
				return
			}
			time.Sleep(time.Second * 1)
			f.status.Speeds = f.status.Downloaded - old
			old = f.status.Downloaded
		}
	}()
}

// 获取下载统计信息
func (f FileDl) GetStatus() Status {
	return f.status
}

// 暂停下载
func (f *FileDl) Pause() {
	f.paused = true
}

// 继续下载
func (f *FileDl) Resume() {
	f.paused = false
	go func() {
		if f.BlockList == nil {
			f.touchOnError(0, errors.New("BlockList == nil, can not get block info"))
			return
		}

		f.touch(f.onResume)
		err := f.download()
		if err != nil {
			f.touchOnError(0, err)
			return
		}
	}()
}

// 任务开始时触发的事件
func (f *FileDl) OnStart(fn func()) {
	f.onStart = fn
}

// 任务暂停时触发的事件
func (f *FileDl) OnPause(fn func()) {
	f.onPause = fn
}

// 任务继续时触发的事件
func (f *FileDl) OnResume(fn func()) {
	f.onResume = fn
}

// 任务完成时触发的事件
func (f *FileDl) OnFinish(fn func()) {
	f.onFinish = fn
}

// 任务出错时触发的事件
// errCode为错误码，errStr为错误描述
func (f *FileDl) OnError(fn func(int, error)) {
	f.onError = fn
}

// 用于触发事件
func (f FileDl) touch(fn func()) {
	if fn != nil {
		go fn()
	}
}

// 触发Error事件
func (f FileDl) touchOnError(errCode int, err error) {
	if f.onError != nil {
		go f.onError(errCode, err)
	}
}



func main() {

	//首先解析命令行参数
	parse_args()

	file, err := os.Create(storeFile)
	if err != nil {
		log.Println(err)
	}
	defer file.Close()

	fileDl, err := NewFileDl("http://"+serverAddr+"/"+requestFile, file, -1)
	if err != nil {
		log.Println(err)
	}

	fileDl.exited = false
	
	var exit = make(chan bool)
	var resume = make(chan bool)
	var pause bool

	var wg sync.WaitGroup

	wg.Add(1)

	fileDl.OnStart(func() {
		fmt.Println("pull start successfully")
		format := "\033[2K\r%v%% [%s] %v Mbps %v\n"
		for {
			status := fileDl.GetStatus()
			var i = float64(status.Downloaded) / float64(fileDl.Size) * 50
			h := strings.Repeat("=", int(i)) +">" +strings.Repeat(" ", 50-int(i))
			select {
			case <-exit:
				fmt.Printf(format, (status.Downloaded*1.000)*100.0/(fileDl.Size*1.000), h, 0, "[FINISH]")
				fmt.Println("download finished")
				wg.Done()
				fileDl.exited = true
				return //结束这个goroutine
			default:
				if !pause {
					time.Sleep(time.Second * 1)
					fmt.Printf(format, (status.Downloaded*1.000)*100.0/(fileDl.Size*1.000), h, status.Speeds*8.0/1000000, "[DOWNLOADING]")
					os.Stdout.Sync()
				} else {
					fmt.Printf(format, (status.Downloaded*1.000)*100.0/(fileDl.Size*1.000), h, 0, "[PAUSE]")
					os.Stdout.Sync()
					<-resume
					pause = false
				}
			}
		}
	})

	fileDl.OnPause(func() {
		pause = true
	})

	fileDl.OnResume(func() {
		resume <- true
	})

	fileDl.OnFinish(func() {
		exit <- true
	})

	fileDl.OnError(func(errCode int, err error) {
		log.Println(errCode, err)
	})

	//fmt.Printf("%+v\n", fileDl)

	//start := time.Now()

	fileDl.Start()

	//time.Sleep(time.Second * 2)
	//fileDl.Pause()测试因特殊情况导致的断点续传的功能,会有什么情况导致断点。。
	//time.Sleep(time.Second * 1)
	//fileDl.Resume()

	wg.Wait()

	//end := time.Now()
	//fmt.Printf( "====下载过程的时间是%d(ms)====\n\n",end.Sub(start).Nanoseconds() / 1000000)
	//计算文件的md5值
	//md5h := md5.New()
    //io.Copy(md5h, file)
    //fmt.Printf("\n====下载的文件的md5值是：%x====\n", md5h.Sum([]byte(""))) //md5

}

func parse_args() {

    flag.StringVar(&serverAddr,"d", "10.12.212.122:7777", "服务器地址：ip:port")
    
    flag.IntVar(&proxyNum,"n", 0, "代理的数目")
    flag.StringVar(&proxyAddrs,"p", "172.16.9.141:8050,172.16.9.141:8090,172.16.9.141:8070", "代理地址:port:id[,port:id...]")
    flag.StringVar(&requestFile,"r", "data_phase_two.dat", "请求的文件位置")
    flag.IntVar(&MaxThread,"t", 150, "最大连接数")
    flag.StringVar(&storeFile,"s", "/data/tegcode/data/data_phase_two.dat", "下载文件的存储位置")
    flag.IntVar(&CacheSize,"csize", 8192, "cache大小的设置")

    flag.Parse()

    //解析代理服务器
    proxyAddr = strings.Split(proxyAddrs,",")

    //如果文件重名
    for Exist(storeFile)==true{
    	storeFile = storeFile + ".1"
    }
   
}

func Exist(filename string) bool {
    _, err := os.Stat(filename)
    return err == nil || os.IsExist(err)
}

